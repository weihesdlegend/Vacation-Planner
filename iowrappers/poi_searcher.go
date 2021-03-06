package iowrappers

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"go.uber.org/zap"
)

const (
	MaxSearchRadius              = 16000               // 10 miles
	MinMapsResultRefreshDuration = time.Hour * 24 * 14 // 14 days
	GoogleSearchHomePageURL      = "https://www.google.com/"
	RequestIdKey                 = "request_id"
)

type PoiSearcher struct {
	mapsClient  MapsClient
	redisClient RedisClient
}

// can also be used as the result of reverse geocoding
type GeocodeQuery struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

var Logger *zap.SugaredLogger

func CreatePoiSearcher(mapsApiKey string, redisUrl *url.URL) *PoiSearcher {
	poiSearcher := PoiSearcher{
		mapsClient:  CreateMapsClient(mapsApiKey),
		redisClient: CreateRedisClient(redisUrl),
	}
	return &poiSearcher
}

func (poiSearcher PoiSearcher) GetMapsClient() *MapsClient {
	return &poiSearcher.mapsClient
}

func (poiSearcher PoiSearcher) GetRedisClient() *RedisClient {
	return &poiSearcher.redisClient
}

func DestroyLogger() {
	_ = Logger.Sync()
}

// currently geocode is equivalent to mapping city and country to latitude and longitude
func (poiSearcher PoiSearcher) GetGeocode(context context.Context, query *GeocodeQuery) (lat float64, lng float64, err error) {
	originalGeocodeQuery := GeocodeQuery{}
	originalGeocodeQuery.City = query.City
	originalGeocodeQuery.Country = query.Country
	var geocodeMissingErr error
	lat, lng, geocodeMissingErr = poiSearcher.redisClient.GetGeocode(context, query)
	if geocodeMissingErr != nil {
		lat, lng, err = poiSearcher.mapsClient.GetGeocode(context, query)
		if err != nil {
			return
		}
		// either redisClient or mapsClient may have corrected location name in the query
		poiSearcher.redisClient.SetGeocode(context, *query, lat, lng, originalGeocodeQuery)
		Logger.Debugf("Geolocation (lat,lng) Cache miss for location %s, %s is %.4f, %.4f",
			query.City, query.Country, lat, lng)
	}
	return
}

func (poiSearcher PoiSearcher) NearbySearch(context context.Context, request *PlaceSearchRequest) ([]POI.Place, error) {
	location := request.Location
	cityAndCountry := strings.Split(location, ",")

	places := make([]POI.Place, 0)
	lat, lng, err := poiSearcher.GetGeocode(context, &GeocodeQuery{
		City:    cityAndCountry[0],
		Country: cityAndCountry[1],
	})
	if logErr(err, utils.LogError) {
		return places, err
	}

	// request.Location is overwritten to lat,lng after the city,country conversion
	request.Location = fmt.Sprintf("%f,%f", lat, lng)

	var cachedPlaces []POI.Place
	cachedPlaces, err = poiSearcher.redisClient.NearbySearch(context, request)
	if err != nil {
		Logger.Error(err)
	}

	Logger.Debugf("[%s] number of results from redis is %d", context.Value(RequestIdKey), len(cachedPlaces))

	// update last search time for the city
	lastSearchTime, cacheMiss := poiSearcher.redisClient.GetMapsLastSearchTime(context, location, request.PlaceCat)

	currentTime := time.Now()
	// use place data from database if the location is known and the data is fresh and we have sufficient data
	if cacheMiss == nil && (currentTime.Sub(lastSearchTime) <= MinMapsResultRefreshDuration && uint(len(cachedPlaces)) >= request.MinNumResults) {
		Logger.Infof("[%s] Using Redis to fulfill request. Place Type: %s", context.Value(RequestIdKey), request.PlaceCat)
		places = append(places, cachedPlaces...)
		return places, nil
	}

	cacheMiss = poiSearcher.redisClient.SetMapsLastSearchTime(context, location, request.PlaceCat, currentTime.Format(time.RFC3339))
	utils.LogErrorWithLevel(cacheMiss, utils.LogError)

	originalSearchRadius := request.Radius

	request.Radius = MaxSearchRadius // use a large search radius whenever we call external maps services

	// initiate a new external search
	newPlaces, mapsNearbySearchErr := poiSearcher.mapsClient.NearbySearch(context, request)
	utils.LogErrorWithLevel(mapsNearbySearchErr, utils.LogError)

	request.Radius = originalSearchRadius // restore search radius

	// update Redis with all the new places obtained
	poiSearcher.UpdateRedis(context, newPlaces)

	// safe-guard on accessing elements in a nil slice
	if len(newPlaces) > 0 {
		places = append(places, newPlaces...)
	}

	if uint(len(places)) < request.MinNumResults {
		Logger.Debugf("Found %d POI results for place type %s, less than requested number of %d",
			len(places), request.PlaceCat, request.MinNumResults)
	}
	if len(places) == 0 {
		Logger.Debugf("No qualified POI result found in the given location %s, radius %d, and place type: %s",
			request.Location, request.Radius, request.PlaceCat)
		Logger.Debug("location may be invalid")
	}
	return places, nil
}

func (poiSearcher PoiSearcher) PlaceDetailsSearch(context context.Context, placeId string) (place POI.Place, err error) {
	return place, nil
}

func (poiSearcher PoiSearcher) UpdateRedis(context context.Context, places []POI.Place) {
	poiSearcher.redisClient.SetPlacesOnCategory(context, places)
	requestId := context.Value(RequestIdKey)
	Logger.Debugf("request:", requestId, "Redis update complete")
}
