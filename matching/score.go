// score design doc: https://bit.ly/2OTuBhM
package matching

import (
	"github.com/weihesdlegend/Vacation-planner/utils"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"math"
)

const (
	AvgRating  = 3.0
	AvgPricing = PriceLevel2
)

func Score(places []Place) float64 {
	if len(places) == 1 {
		return singlePlaceScore(places[0])
	}
	distances := calDistances(places)                     // Haversine distances
	maxDist := math.Max(0.001, calMaxDistance(distances)) // protect against maximum distance being zero
	avgDistance := stat.Mean(distances, nil) / maxDist    // normalized average distance

	avgScore := avgPlacesScore(places)

	return avgScore - avgDistance
}

func ScoreNoDistance(places []Place) float64 {
	if len(places) == 1 {
		return singlePlaceScore(places[0])
	}
	res := float64(0)
	for _, place:= range places{
		res += singlePlaceScoreKnapSack(place)
	}
	return res
}

func scaleScoreNoCompensation(place Place) float64 {
	var res float64
	var placeRating float64
	placeRating = float64(place.GetRating())
	res = math.Log10(1.25 + float64(place.GetUserRatingsCount())) * placeRating
	//fmt.Printf("Place %s, score: %f\n",place.Place.Name, res)
	return res
}

func singlePlaceScoreKnapSack(place Place) float64 {
	return scaleScoreNoCompensation(place)
}

func singlePlaceScore(place Place) float64 {
	var ratingPricingRatio float64
	if place.GetPrice() == 0 {
		ratingPricingRatio = AvgRating / AvgPricing // set to average single Place rating-price ratio
	} else {
		ratingPricingRatio = float64(place.GetRating()) / place.GetPrice()
	}
	return math.Log10(float64(1+place.GetUserRatingsCount())) * ratingPricingRatio
}

// calculate Haversine distances between places
func calDistances(places []Place) []float64 {
	distances := make([]float64, len(places)-1)

	for i := 0; i < len(distances); i++ {
		locationX := places[i].GetLocation()
		locationY := places[i+1].GetLocation()
		distances[i] = utils.HaversineDist([]float64{locationX[0], locationX[1]}, []float64{locationY[0], locationY[1]})
	}
	return distances
}

func calMaxDistance(distances []float64) float64 {
	return floats.Max(distances)
}

// calculate normalized average rating to price ratio
func avgPlacesScore(places []Place) float64 {
	numPlaces := len(places)
	placeScores := make([]float64, numPlaces)
	for k, place := range places {
		placeScores[k] = singlePlaceScore(place)
	}
	return stat.Mean(placeScores, nil)
}
