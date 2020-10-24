package iowrappers

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"googlemaps.github.io/maps"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	GoogleNearbySearchDelay = time.Second
	GoogleMapsSearchTimeout = time.Second * 10
)

var detailedSearchFields = flag.String("fields", "name,opening_hours,formatted_address,adr_address,url", "a list of comma-separated fields")

// request generated by clustering layer
type PlaceSearchRequest struct {
	// "city,country"
	Location string
	// "visit", "eatery",...
	PlaceCat POI.PlaceCategory
	// search radius
	Radius uint
	// rank by
	RankBy string
	// maximum number of results, set this upper limit for reducing upper-layer computational load and limiting external API call
	MaxNumResults uint
	// minimum number of results, set this lower limit for reducing risk of zero result in upper-layer computations
	MinNumResults uint
}

func GoogleMapsNearbySearchWrapper(c MapsClient, location string, placeType string, radius uint,
	pageToken string, rankBy string) (resp maps.PlacesSearchResponse, err error) {
	latLng, err := maps.ParseLatLng(location)
	// since we try to use Redis and database before calling nearby search,
	// if location cannot be parsed, then the request cannot be fulfilled.
	if logErr(err, utils.LogError) {
		return
	}

	mapsReq := maps.NearbySearchRequest{
		Type:      maps.PlaceType(placeType),
		Location:  &latLng,
		Radius:    radius,
		PageToken: pageToken,
		RankBy:    maps.RankBy(rankBy),
	}
	resp, err = c.client.NearbySearch(context.Background(), &mapsReq)
	logErr(err, utils.LogError)
	return
}

func (mapsClient *MapsClient) NearbySearch(c context.Context, request *PlaceSearchRequest) ([]POI.Place, error) {
	var maxReqTimes uint = 5
	var places = make([]POI.Place, 0)
	var searchDone = make(chan bool)
	ctx, cancelFunc := context.WithTimeout(c, GoogleMapsSearchTimeout)
	defer cancelFunc()

	go mapsClient.ExtensiveNearbySearch(ctx, maxReqTimes, request, &places, searchDone)

	select {
	case <-searchDone:
		return places, nil
	case <-ctx.Done():
		return places, errors.New("maps search time out")
	}
}

func (mapsClient *MapsClient) PlaceDetailsSearch(context.Context, string) (place POI.Place, err error) {
	return
}

// ExtensiveNearbySearch attempts to find a specified number of places satisfy the request
// within the maxRequestTime times of calling external APIs
func (mapsClient *MapsClient) ExtensiveNearbySearch(context context.Context, maxRequestTimes uint, request *PlaceSearchRequest, places *[]POI.Place, done chan bool) {
	if request.RankBy == "" {
		request.RankBy = "prominence" // default rankBy value
	}

	placeTypes := POI.GetPlaceTypes(request.PlaceCat) // get place types in a category

	nextPageTokenMap := make(map[POI.LocationType]string) // map for place type to search token
	for _, placeType := range placeTypes {
		nextPageTokenMap[placeType] = ""
	}

	var reqTimes uint = 0    // number of queries for each location type
	var totalResult uint = 0 // number of results so far, keep this number low

	microAddrMap := make(map[string]string) // map place ID to its micro-address
	placeMap := make(map[string]bool)       // remove duplication for place with same ID
	urlMap := make(map[string]string)       // map place ID to url

	searchStartTime := time.Now()

	var err error
	for totalResult < request.MinNumResults {
		// if error, return regardless of number of results obtained
		if err != nil {
			done <- true
			return
		}
		for _, placeType := range placeTypes {
			if reqTimes > 0 && nextPageTokenMap[placeType] == "" { // no more result for this location type
				continue
			}

			nextPageToken := nextPageTokenMap[placeType]
			searchResp, error_ := GoogleMapsNearbySearchWrapper(*mapsClient, request.Location, string(placeType), request.Radius, nextPageToken, request.RankBy)
			if error_ != nil {
				err = error_
				Logger.Error(err)
				continue
			}

			placeIdMap := make(map[int]string) // maps index in search response to place ID
			for k, res := range searchResp.Results {
				if res.OpeningHours == nil || res.OpeningHours.WeekdayText == nil {
					placeIdMap[k] = res.PlaceID
				}
			}

			detailSearchResults := make([]PlaceDetailSearchRes, len(placeIdMap))
			var wg sync.WaitGroup
			wg.Add(len(placeIdMap))
			for idx, placeId := range placeIdMap {
				go PlaceDetailsSearchWrapper(mapsClient, idx, placeId, &detailSearchResults[idx], &wg)
			}
			wg.Wait()

			// fill fields from detail search results to nearby search results
			for _, placeDetails := range detailSearchResults {
				searchRespIdx := placeDetails.RespIdx
				placeId := searchResp.Results[searchRespIdx].PlaceID
				searchResp.Results[searchRespIdx].OpeningHours = placeDetails.Res.OpeningHours
				searchResp.Results[searchRespIdx].FormattedAddress = placeDetails.Res.FormattedAddress
				microAddrMap[placeId] = placeDetails.Res.AdrAddress
				urlMap[placeId] = placeDetails.Res.URL
			}

			*places = append(*places, parsePlacesSearchResponse(searchResp, placeType, microAddrMap, placeMap, urlMap)...)
			totalResult += uint(len(searchResp.Results))
			nextPageTokenMap[placeType] = searchResp.NextPageToken
		}
		reqTimes++
		if reqTimes == maxRequestTimes {
			break
		}
		time.Sleep(GoogleNearbySearchDelay) // sleep to make sure new next page token comes to effect
	}

	searchDuration := time.Since(searchStartTime)

	// logging
	Logger.Infow("Logging nearby search",
		"Maps API call time", searchDuration,
		"center location (lat,lng)", request.Location,
		"place category", request.PlaceCat,
		"total results", totalResult,
	)
	done <- true
	return
}

type PlaceDetailSearchRes struct {
	Res     *maps.PlaceDetailsResult
	RespIdx int
}

func PlaceDetailsSearchWrapper(mapsClient *MapsClient, idx int, placeId string, detailSearchRes *PlaceDetailSearchRes, wg *sync.WaitGroup) {
	defer wg.Done()
	searchRes, err := PlaceDetailedSearch(mapsClient, placeId)
	if err != nil {
		Logger.Error(err)
		return
	}
	*detailSearchRes = PlaceDetailSearchRes{Res: &searchRes, RespIdx: idx}
	return
}

func PlaceDetailedSearch(mapsClient *MapsClient, placeId string) (maps.PlaceDetailsResult, error) {
	if reflect.ValueOf(mapsClient).IsNil() {
		err := errors.New("client does not exist")
		Logger.Error(err)
		return maps.PlaceDetailsResult{}, err
	}
	flag.Parse() // parse detailed search fields
	req := &maps.PlaceDetailsRequest{
		PlaceID: placeId,
	}
	if *detailedSearchFields != "" {
		fieldMask, err := parseFields(*detailedSearchFields)
		utils.CheckErrImmediate(err, utils.LogError)
		req.Fields = fieldMask
	}

	startSearchTime := time.Now()
	resp, err := mapsClient.client.PlaceDetails(context.Background(), req)
	utils.CheckErrImmediate(err, utils.LogError)

	searchDuration := time.Since(startSearchTime)

	// logging
	Logger.Debugw("Logging detailed place search",
		"Maps API call time", searchDuration,
		"place name", resp.Name,
		"place formatted address", resp.FormattedAddress,
	)

	return resp, err
}

func parsePlacesSearchResponse(resp maps.PlacesSearchResponse, locationType POI.LocationType, microAddrMap map[string]string, placeMap map[string]bool, urlMap map[string]string) (places []POI.Place) {
	for _, res := range resp.Results {
		id := res.PlaceID
		if seen, _ := placeMap[id]; seen {
			continue
		} else {
			placeMap[id] = true
		}
		name := res.Name
		lat := fmt.Sprintf("%f", res.Geometry.Location.Lat)
		lng := fmt.Sprintf("%f", res.Geometry.Location.Lng)
		location := strings.Join([]string{lat, lng}, ",")
		addr := ""
		if microAddrMap != nil {
			addr = microAddrMap[id]
		}
		priceLevel := res.PriceLevel
		h := &POI.OpeningHours{}
		if res.OpeningHours != nil && res.OpeningHours.WeekdayText != nil && len(res.OpeningHours.WeekdayText) > 0 {
			h.Hours = append(h.Hours, res.OpeningHours.WeekdayText...)
		}
		rating := res.Rating
		url := urlMap[id]
		var photo *maps.Photo
		if len(res.Photos) > 0 {
			photo = &res.Photos[0]
		}
		places = append(places, POI.CreatePlace(name, location, addr, res.FormattedAddress, locationType, h, id, priceLevel, rating, url, photo))
	}
	return
}

// refs: maps/examples/places/placedetails/placedetails.go
func parseFields(fields string) ([]maps.PlaceDetailsFieldMask, error) {
	var res []maps.PlaceDetailsFieldMask
	for _, s := range strings.Split(fields, ",") {
		f, err := maps.ParsePlaceDetailsFieldMask(s)
		if logErr(err, utils.LogError) {
			return res, err
		}
		res = append(res, f)
	}
	return res, nil
}

func logErr(err error, logLevel uint) bool {
	return utils.CheckErrImmediate(err, logLevel)
}
