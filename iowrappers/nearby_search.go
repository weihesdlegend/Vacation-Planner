package iowrappers

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"context"
	"flag"
	"fmt"
	"errors"
	"googlemaps.github.io/maps"
	"strings"
	"time"
	log "github.com/sirupsen/logrus"
)

type LocationType string

const(
	LocationTypeCafe = LocationType("cafe")
	LocationTypeRestaurant = LocationType("restaurant")
	LocationTypeMuseum = LocationType("museum")
	LocationTypeGallery = LocationType("art_gallery")
	LocationTypeAmusementPark = LocationType("amusement_park")
	LocationTypePark = LocationType("park")
)

const(
	GOOGLE_NEARBY_SEARCH_DELAY = time.Duration(2 * time.Second)
)

var detailedSearchFields = flag.String("fields", "name,opening_hours,formatted_address,adr_address", "a list of comma-separated fields")

// request generated by clustering layer
type PlaceSearchRequest struct{
	// "lat,lng"
	Location string
	// "visit", "eatery",...
	PlaceCat POI.PlaceCategory
	// search radius
	Radius uint
	// rank by
	RankBy string
	// number of results
	MaxNumResults uint
}

func GoogleNearbySearchSDK(c MapsClient, location string, placeType string, radius uint,
	pageToken string, rankBy string) (resp maps.PlacesSearchResponse){
	var err error
	latlng, err := maps.ParseLatLng(location)
	utils.CheckErr(err)

	mapsReq := maps.NearbySearchRequest{
		Type: maps.PlaceType(placeType),
		Location: &latlng,
		Radius: radius,
		PageToken:pageToken,
		RankBy: maps.RankBy(rankBy),
	}
	resp, err = c.client.NearbySearch(context.Background(), &mapsReq)
	utils.CheckErr(err)
	return
}

func (c *MapsClient) NearbySearch(request *PlaceSearchRequest)(places []POI.Place){
	var maxReqTimes uint = 5
	return c.ExtensiveNearbySearch(maxReqTimes, request)
}

// ExtensiveNearbySearch tries to find a specified number of search results from a place category once for each location type in the category
// maxRequestTime specifies the number of times to query for each location type having maxRequestTimes provides Google API call protection
func (c *MapsClient) ExtensiveNearbySearch(maxRequestTimes uint, request *PlaceSearchRequest) (places []POI.Place) {
	if request.RankBy == ""{
		request.RankBy = "prominence"	// default rankBy value
	}

	placeTypes := getTypes(request.PlaceCat)	// get place types in a category

	nextPageTokenMap := make(map[LocationType]string)	// map for place type to search token
	for _, placeType := range placeTypes{
		nextPageTokenMap[placeType] = ""
	}

	var reqTimes uint = 0		// number of queries for each location type
	var totalResult uint= 0	// number of results so far, keep this number low

	microAddrMap := make(map[string]string)	// map place ID to its micro-address

	searchStartTime := time.Now()

	for totalResult <= request.MaxNumResults && reqTimes < maxRequestTimes{
		for _, placeType := range placeTypes{
			if reqTimes > 0 && nextPageTokenMap[placeType] == ""{	// no more result for this location type
				continue
			}
			nextPageToken := nextPageTokenMap[placeType]
			searchResp := GoogleNearbySearchSDK(*c, request.Location, string(placeType), request.Radius, nextPageToken, request.RankBy)
			for k, res := range searchResp.Results{
				if res.OpeningHours == nil || res.OpeningHours.WeekdayText == nil{
					detailedSearchRes, _ := c.PlaceDetailedSearch(res.PlaceID)
					searchResp.Results[k].OpeningHours = detailedSearchRes.OpeningHours
					searchResp.Results[k].FormattedAddress = detailedSearchRes.FormattedAddress
					microAddrMap[searchResp.Results[k].ID] = detailedSearchRes.AdrAddress	// assume ID is always available
				}
			}
			places = append(places, parsePlacesSearchResponse(searchResp, placeType, microAddrMap)...)
			totalResult += uint(len(searchResp.Results))
			nextPageTokenMap[placeType] = searchResp.NextPageToken
			reqTimes++
		}
		time.Sleep(GOOGLE_NEARBY_SEARCH_DELAY)	// sleep to make sure new next page token comes to effect
	}

	searchDuration := time.Since(searchStartTime)

	// logging
	c.logger.WithFields(log.Fields{
		"center location (lat/lng)": request.Location,
		"place category": request.PlaceCat,
		"total results": totalResult,
		"Maps API call time": searchDuration,
	}).Info("Logging nearby search")

	return
}

func (c *MapsClient) PlaceDetailedSearch(placeId string) (maps.PlaceDetailsResult, error){
	if c.client == nil{
		return maps.PlaceDetailsResult{}, errors.New("client does not exist")
	}
	flag.Parse()	// parse detailed search fields

	req := &maps.PlaceDetailsRequest{
		PlaceID: placeId,
	}

	if *detailedSearchFields != ""{
		fieldMask, err := parseFields(*detailedSearchFields)
		utils.CheckErr(err)
		req.Fields = fieldMask
	}

	startSearchTime := time.Now()

	resp, err := c.client.PlaceDetails(context.Background(), req)

	searchDuration := time.Since(startSearchTime)

	// logging
	c.logger.WithFields(log.Fields{
		"place name": resp.Name,
		"place formatted address": resp.FormattedAddress,
		"Maps API call time": searchDuration,
	}).Info("Logging detailed place search")

	utils.CheckErr(err)

	return resp, nil
}

func parsePlacesSearchResponse(resp maps.PlacesSearchResponse, locationType LocationType, microAddrMap map[string]string) (places []POI.Place) {
	for _, res := range resp.Results{
		name := res.Name
		lat := fmt.Sprintf("%f", res.Geometry.Location.Lat)
		lng := fmt.Sprintf("%f", res.Geometry.Location.Lng)
		location := strings.Join([]string{lat, lng}, ",")
		addr := ""
		if microAddrMap != nil {
			addr = microAddrMap[res.ID]
		}
		id := res.PlaceID
		priceLevel := res.PriceLevel
		h := &POI.OpeningHours{}
		if res.OpeningHours != nil && res.OpeningHours.WeekdayText != nil && len(res.OpeningHours.WeekdayText) > 0{
			h.Hours = append(h.Hours, res.OpeningHours.WeekdayText...)
		}
		places = append(places, POI.CreatePlace(name, location, addr, res.FormattedAddress, string(locationType), h, id, priceLevel))
	}
	return
}

// Given a location type returns a set of types defined in google maps API
func getTypes (placeCat POI.PlaceCategory) (placeTypes []LocationType){
	switch placeCat{
	case POI.PlaceCategoryVisit:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypePark, LocationTypeAmusementPark, LocationTypeGallery, LocationTypeMuseum}...)
	case POI.PlaceCategoryEatery:
		placeTypes = append(placeTypes,
			[]LocationType{LocationTypeCafe, LocationTypeRestaurant}...)
	}
	return
}

// refs: maps/examples/places/placedetails/placedetails.go
func parseFields(fields string) ([]maps.PlaceDetailsFieldMask, error) {
	var res []maps.PlaceDetailsFieldMask
	for _, s := range strings.Split(fields, ",") {
		f, err := maps.ParsePlaceDetailsFieldMask(s)
		utils.CheckErr(err)
		res = append(res, f)
	}
	return res, nil
}
