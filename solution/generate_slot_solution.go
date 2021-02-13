package solution

import (
	"container/heap"
	"context"
	"errors"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"strconv"
)

const (
	CandidateQueueLength                  = 15
	ReqTimeSlotsTagMismatchErrMsg         = "user designated stay times list length does not match tag length"
	CategorizedPlaceIterInitFailureErrMsg = "categorized places iterator init failure"
)

// Find top solution candidates
func FindBestCandidates(candidates []SlotSolutionCandidate) []SlotSolutionCandidate {
	m := make(map[string]SlotSolutionCandidate) // map for result extraction
	vertexes := make([]graph.Vertex, len(candidates))
	for idx, candidate := range candidates {
		candidateKey := strconv.FormatInt(int64(idx), 10)
		vertex := graph.Vertex{Name: candidateKey, Key: candidate.Score}
		vertexes[idx] = vertex
		m[candidateKey] = candidate
	}
	// use limited-size minimum priority queue
	priorityQueue := &graph.MinPriorityQueueVertex{}
	for _, vertex := range vertexes {
		if priorityQueue.Len() == CandidateQueueLength {
			top := (*priorityQueue)[0]
			if vertex.Key > top.Key {
				heap.Pop(priorityQueue)
			} else {
				continue
			}
		}
		heap.Push(priorityQueue, vertex)
	}

	res := make([]SlotSolutionCandidate, 0)

	for priorityQueue.Len() > 0 {
		top := heap.Pop(priorityQueue).(graph.Vertex)
		res = append(res, m[top.Name])
	}

	return res
}

func generateCategorizedPlaces(context context.Context, timeMatcher *matching.TimeMatcher, location string, radius uint, weekday POI.Weekday, timeSlots []matching.TimeSlot) ([]CategorizedPlaces, int) {
	matchingRequest := &matching.TimeMatchingRequest{
		Location:  location,
		Radius:    radius,
		TimeSlots: timeSlots,
		Weekday:   weekday,
	}
	timePlaceClusters := timeMatcher.Matching(context, matchingRequest)
	// one set of categorized places for each TimePlaceCluster
	categorizedPlaces := make([]CategorizedPlaces, len(timePlaceClusters))

	// place clusters are clustered by time slot
	// now cluster by place category
	for idx, timePlaceCluster := range timePlaceClusters {
		categorizedPlaces[idx] = Categorize(timePlaceCluster)
	}
	return categorizedPlaces, GetTimeSlotLengthInMin(timePlaceClusters)
}

// Generate slot solution candidates
// Parameter list matches slot request
func GenerateSlotSolution(context context.Context, timeMatcher *matching.TimeMatcher, location string, evTag string, stayTimes []matching.TimeSlot, radius uint, weekday POI.Weekday, redisClient iowrappers.RedisClient, redisReq iowrappers.SlotSolutionCacheRequest) (slotSolution SlotSolution, slotSolutionRedisKey string, err error) {
	if len(stayTimes) != len(evTag) {
		err = errors.New(ReqTimeSlotsTagMismatchErrMsg)
		return
	}

	err = slotSolution.SetTag(evTag)
	if err != nil {
		return
	}

	slotSolution.SlotSolutionCandidates = make([]SlotSolutionCandidate, 0)
	slotCandidates := make([]SlotSolutionCandidate, 0)

	if radius <= 0 {
		radius = 2000
	}

	categorizedPlaces, minuteLimit := generateCategorizedPlaces(context, timeMatcher, location, radius, weekday, stayTimes)

	mdIter := MDtagIter{}
	if !mdIter.Init(evTag, categorizedPlaces) {
		err = errors.New(CategorizedPlaceIterInitFailureErrMsg)
		return
	}

	for mdIter.HasNext() {
		curCandidate := slotSolution.CreateCandidate(mdIter, categorizedPlaces)

		if curCandidate.IsSet {
			_, travelTimeInMin := GetTravelTimeByDistance(categorizedPlaces, mdIter)
			if travelTimeInMin <= float64(minuteLimit) {
				slotCandidates = append(slotCandidates, curCandidate)
			}
		}
		mdIter.Next()
	}
	bestCandidates := FindBestCandidates(slotCandidates)
	slotSolution.SlotSolutionCandidates = append(slotSolution.SlotSolutionCandidates, bestCandidates...)

	// cache slot solution calculation results
	slotSolutionToCache := iowrappers.SlotSolutionCacheResponse{}
	slotSolutionToCache.SlotSolutionCandidate = make([]iowrappers.SlotSolutionCandidateCache, len(slotSolution.SlotSolutionCandidates))

	for idx, slotSolutionCandidate := range slotSolution.SlotSolutionCandidates {
		candidateCache := iowrappers.SlotSolutionCandidateCache{
			PlaceIds:       slotSolutionCandidate.PlaceIDS,
			Score:          slotSolutionCandidate.Score,
			PlaceNames:     slotSolutionCandidate.PlaceNames,
			PlaceLocations: slotSolutionCandidate.PlaceLocations,
			PlaceAddresses: slotSolutionCandidate.PlaceAddresses,
			PlaceURLs:      slotSolutionCandidate.PlaceURLs,
		}
		slotSolutionToCache.SlotSolutionCandidate[idx] = candidateCache
	}

	redisClient.CacheSlotSolution(context, redisReq, slotSolutionToCache)

	return
}

// NearbySearchWithPlaceView returns PlaceView results for single day nearby search with a fixed time slot range
func NearbySearchWithPlaceView(context context.Context, timeMatcher *matching.TimeMatcher, location string,
	weekday POI.Weekday, radius uint, timeSlot matching.TimeSlot, category POI.PlaceCategory) ([]PlaceView, error) {
	timeSlots := []matching.TimeSlot{timeSlot}
	categorizedPlaces, _ := generateCategorizedPlaces(context, timeMatcher, location, radius, weekday, timeSlots)
	if len(categorizedPlaces) != 1 {
		return nil, errors.New("we should only get one set of categorized places")
	}

	var places []matching.Place
	switch category {
	case POI.PlaceCategoryEatery:
		places = categorizedPlaces[0].EateryPlaces
	case POI.PlaceCategoryVisit:
		places = categorizedPlaces[0].VisitPlaces
	}

	var placesView = make([]PlaceView, len(places))
	for idx, place := range places {
		placesView[idx] = ToPlaceView(place)
	}
	return placesView, nil
}

// NearbySearchAllCategories returns places results for a single day with a fixed time slot range
func NearbySearchAllCategories(context context.Context, timeMatcher *matching.TimeMatcher, location string,
	weekday POI.Weekday, radius uint, timeSlot matching.TimeSlot) ([]matching.Place, error) {
	timeSlots := []matching.TimeSlot{timeSlot}
	categorizedPlaces, _ := generateCategorizedPlaces(context, timeMatcher, location, radius, weekday, timeSlots)
	var err error
	if len(categorizedPlaces) != 1 {
		return nil, errors.New("we should only get one set of categorized places")
	}
	var places []matching.Place
	places = append(places, categorizedPlaces[0].EateryPlaces...)
	places = append(places, categorizedPlaces[0].VisitPlaces...)
	if len(places) == 0 {
		err = errors.New("no places found at current location and time")
	}
	return places, err
}
