package matching

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/graph"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

//location-based clustering, can be extended to matcher for multi-city planning
type PlacePair struct {
	VisitName  string
	EateryName string
	PairPrice  float64
}

type LocationMatcher struct {
	EateryClusterMgr *graph.ClustersManager
	VisitClusterMgr  *graph.ClustersManager
}

type LocationMatchingRequest struct {
	Location    string
	Radius      uint
	NumClusters uint
}

func (m *LocationMatcher) Matching(req *LocationMatchingRequest, mapsClient *iowrappers.MapsClient) (placePairs []PlacePair) {
	//m.createClusterManager()

	/*
		1) get geo location data
		2) clustering
		3) calculate cluster centers
		4) generate Place pairs
	*/
	m.createClusterManager(mapsClient, req.NumClusters)

	m.clustering(req.Location, req.Radius, req.NumClusters)

	// pair cluster centers from two categories
	clusterPairs, err := MatchClusterCenters(*m.VisitClusterMgr.ClusterCenters, *m.EateryClusterMgr.ClusterCenters)

	utils.LogErrorWithLevel(err, utils.LogError)

	for _, clusterPair := range clusterPairs {
		placePairs = append(placePairs, m.genPlacePairs(&clusterPair)...)
	}
	return
}

func (m *LocationMatcher) genPlacePairs(clusterPair *clusterCenterPair) (placePairs []PlacePair) {
	visitCluster := m.VisitClusterMgr.PlaceClusters.Clusters[clusterPair.VisitIdx]
	eateryCluster := m.EateryClusterMgr.PlaceClusters.Clusters[clusterPair.EateryIdx]

	for _, visitPlace := range visitCluster.Places {
		for _, eateryPlace := range eateryCluster.Places {
			price := Pricing(visitPlace.GetPriceLevel()) +
				Pricing(eateryPlace.GetPriceLevel())
			placePairs = append(placePairs,
				PlacePair{visitPlace.GetName(), eateryPlace.GetName(), price})
		}
	}
	return
}

func (m *LocationMatcher) createClusterManager(mapsClient *iowrappers.MapsClient, numClusters uint) {
	m.EateryClusterMgr = &graph.ClustersManager{}
	m.EateryClusterMgr.Init(mapsClient, POI.PlaceCategoryEatery, numClusters)
	m.VisitClusterMgr = &graph.ClustersManager{}
	m.VisitClusterMgr.Init(mapsClient, POI.PlaceCategoryVisit, numClusters)
}

func (m *LocationMatcher) clustering(location string, searchRadius uint, numClusters uint) {
	// get geolocation data for visit places
	visitData := m.VisitClusterMgr.PlaceSearch(location, searchRadius, "")
	// clustering for visit places
	visitClusterResult, visitClusterSizes := m.VisitClusterMgr.Clustering(&visitData, int(numClusters))
	// calculate cluster centers
	m.VisitClusterMgr.FindClusterCenter(&visitData, &visitClusterResult, &visitClusterSizes)

	// get geolocation data for visit places
	eateryData := m.EateryClusterMgr.PlaceSearch(location, searchRadius, "")
	// clustering for eatery places
	eateryClusterResult, eateryClusterSizes := m.EateryClusterMgr.Clustering(&eateryData, int(numClusters))
	// calculate cluster centers
	m.EateryClusterMgr.FindClusterCenter(&eateryData, &eateryClusterResult, &eateryClusterSizes)
}
