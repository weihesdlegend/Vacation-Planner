package test

import (
	"Vacation-planner/POI"
	"testing"
)

func TestCreatePlace(t *testing.T){
	location := "32.715736,-117.161087"
	name := "lincoln park"
	addr := "450 National Ave, Mountain View, USA, 94043"
	place := POI.CreatePlace(name, location, addr, "stay", "lincolnpark_mtv" )
	if place.GetName() != name{
		t.Errorf("Name setting is not correct. \n Expected: %s, got: %s",
			name, place.GetName())
	}
	if place.GetLocation() != [2]float64{32.715736,-117.161087}{
		t.Errorf("Location setting is not correct.")
	}
	if place.GetType() != "stay" {
		t.Errorf("Type setting is not correct.")
	}
	if place.GetAddress() != addr{
		t.Errorf("Address setting is not correct. \n Expected: %s \n Got: %s",
			addr, place.GetAddress())
	}
}