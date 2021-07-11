package knapsack

import (
	"github.com/stretchr/testify/assert"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"testing"
)

func TestKnapsack(t *testing.T) {
	places := make([]matching.Place, 20, 20)
	placesFromData := make([]POI.Place, 20, 20)
	err := utils.ReadFromFile("data/random_gen_visiting_places_for_test.json", &placesFromData)
	if err != nil || len(places) == 0 {
		t.Fatal("Json file read error")
	}
	for idx, p := range placesFromData {
		if idx >= len(places) {
			break
		}
		places[idx] = matching.CreatePlace(p, POI.PlaceCategoryVisit)
	}
	budget := uint(80)
	querystart := matching.QueryTimeInterval{StartHour:8, Day:POI.DateMonday, EndHour:16}
	timeLimit := querystart.EndHour - querystart.StartHour
	result := matching.KnapsackV1(places, querystart, budget)
	if len(result) == 0 {
		t.Error("No result is returned.")
	}
	result2, totalCost, totalTimeSpent, err := matching.Knapsack(places, querystart, budget)
	if err != nil {
		t.Error("Error message: #{err.Error()}")
	}

	t.Logf("total cost of the trip is %d", totalCost)
	t.Logf("total time of the trip is %d", totalTimeSpent)

	assert.LessOrEqual(t, totalTimeSpent, timeLimit, "")
	assert.LessOrEqual(t, totalCost, budget, "")

	if len(result) == 0 {
		t.Error("No result is returned by v2")
	}
	for _, p := range result {
		t.Logf("Placename: %s, ID: %s", p.GetPlaceName(), p.GetPlaceId())
	}
	t.Logf("Knapsack V1 result size: %d", len(result))
	for _, p := range result2 {
		t.Logf("Placename: %s, ID: %s", p.GetPlaceName(), p.GetPlaceId())
	}
	t.Logf("Knapsack V2 result size: %d", len(result2))
	if len(result) != len(result2) {
		t.Error("v2 result doesn't match")
	}
	for i := range result {
		if result[i].GetPlaceId() != result2[i].GetPlaceId() {
			t.Error("v2 result is not the same")
		}
	}
	assert.Equal(t, "ChIJ36yUcg3xNIgRtvNioeVfK7E", result2[0].GetPlaceId())
}
