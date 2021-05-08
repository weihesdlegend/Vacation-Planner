package graph

import (
	"github.com/mpraski/clusters"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

type BasicCluster struct {
	Places []POI.Place
}

// Size of BasicCluster returns number of Places in a cluster
func (cluster *BasicCluster) Size() int {
	return len(cluster.Places)
}

type PlaceClusters struct {
	Clusters []BasicCluster
}

// Size of PlaceClusters returns number of Clusters in a zone
func (placeClusters *PlaceClusters) Size() int {
	return len(placeClusters.Clusters)
}

type ClustersManager struct {
	Client         *iowrappers.MapsClient
	PlaceClusters  *PlaceClusters
	places         []POI.Place
	PlaceCat       POI.PlaceCategory
	ClusterCenters *[][]float64 // TODO: cluster centers should be within cluster as an attribute
}

func (placeManager *ClustersManager) Init(mapsClient *iowrappers.MapsClient, placeCat POI.PlaceCategory, numClusters uint) {
	placeManager.Client = mapsClient
	placeManager.PlaceClusters = &PlaceClusters{}
	placeManager.PlaceClusters.Clusters = make([]BasicCluster, numClusters)
	placeManager.PlaceCat = placeCat
	clusterCenters := make([][]float64, numClusters)
	placeManager.ClusterCenters = &clusterCenters
}

// call Google API to obtain nearby Places and extract location data
func (placeManager *ClustersManager) PlaceSearch(location string, searchRadius uint, searchType string) [][]float64 {
	request := iowrappers.PlaceSearchRequest{
		Location: location,
		PlaceCat: placeManager.PlaceCat,
		Radius:   searchRadius,
		RankBy:   "prominence",
	}
	if searchType == "" {
		request.MinNumResults = 20
	} else {
		request.MinNumResults = 100
	}
	placeManager.places, _ = placeManager.Client.NearbySearch(nil, &request)

	locationData := make([][]float64, len(placeManager.places))
	for idx, place := range placeManager.places {
		latLng := place.GetLocation()
		locationData[idx] = []float64{latLng[0], latLng[1]}
	}
	return locationData
}

// train clustering model and assign Places to Clusters
// numClusters specifies number of Clusters
func (placeManager *ClustersManager) Clustering(geoLocationData *[][]float64, numClusters int) (clusterResult []int, clusterSizes []int) {
	// obtain clusterer with number of Clusters and distance function
	hardCluster, err := clusters.KMeans(1000, numClusters, utils.HaversineDist)
	utils.LogErrorWithLevel(err, utils.LogError)

	// training
	err = hardCluster.Learn(*geoLocationData)
	utils.LogErrorWithLevel(err, utils.LogError)

	// save membership info
	for locationIdx, clusterIdx := range hardCluster.Guesses() {
		curCluster := &placeManager.PlaceClusters.Clusters[clusterIdx-1]
		curCluster.Places = append(curCluster.Places, placeManager.places[locationIdx])
	}

	clusterResult = hardCluster.Guesses()
	clusterSizes = hardCluster.Sizes()
	return
}

func (placeManager *ClustersManager) FindClusterCenter(geoLocationData *[][]float64, clusterResult *[]int,
	clusterSizes *[]int) [][]float64 {
	clusterCenters := make([][]float64, placeManager.PlaceClusters.Size())

	groups := make([][][]float64, placeManager.PlaceClusters.Size())

	for i := 0; i < placeManager.PlaceClusters.Size(); i++ {
		groups[i] = [][]float64{}
	}

	for k, cluster := range *clusterResult {
		groups[cluster-1] = append(groups[cluster-1], (*geoLocationData)[k])
	}

	for i := 0; i < placeManager.PlaceClusters.Size(); i++ {
		center, err := utils.FindCenter(groups[i])
		utils.LogErrorWithLevel(err, utils.LogError)
		clusterCenters[i] = center
	}
	placeManager.ClusterCenters = &clusterCenters
	return clusterCenters
}
