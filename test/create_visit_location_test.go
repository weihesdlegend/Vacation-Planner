package test

import (
	"github.com/weihesdlegend/Vacation-planner/POI"
	"testing"
)

func TestCreatePlace(t *testing.T) {
	location := "32.715736,-117.161087"
	name := "lincoln park"
	addr := "450 National Ave, Mountain View, USA, 94043"
	place := POI.CreatePlace(name, location, addr, addr, "stay", nil, "lincolnpark_mtv", 3, 4.5)
	if place.GetName() != name {
		t.Errorf("Name setting is not correct. \n Expected: %s, got: %s",
			name, place.GetName())
	}
	if place.GetLocation() != [2]float64{-117.161087, 32.715736} {
		t.Errorf("Location setting is not correct.")
	}
	if place.GetType() != "stay" {
		t.Errorf("Type setting is not correct.")
	}
	if place.GetFormattedAddress() != addr {
		t.Errorf("Address setting is not correct. \n Expected: %s \n Got: %s",
			addr, place.GetAddress())
	}
	if place.GetPriceLevel() != 3 {
		t.Errorf("Price level setting is not correct. \n Expected: %d \n Got: %d",
			3, place.GetPriceLevel())
	}
	if place.GetRating() != 4.5 {
		t.Errorf("Price rating setting is not correct. \n Expected: %f \n Got: %f	",
			4.5, place.GetRating())
	}
}
