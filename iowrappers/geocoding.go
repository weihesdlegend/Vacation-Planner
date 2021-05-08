package iowrappers

import (
	"context"
	"errors"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"googlemaps.github.io/maps"
)

// Translate city, country to its central location
func (mapsClient MapsClient) GetGeocode(ctx context.Context, query *GeocodeQuery) (lat float64, lng float64, err error) {
	req := &maps.GeocodingRequest{
		Components: map[maps.Component]string{
			maps.ComponentLocality: query.City,
			maps.ComponentCountry:  query.Country,
		}}

	resp, err := mapsClient.client.Geocode(ctx, req)
	if err != nil {
		utils.LogErrorWithLevel(err, utils.LogError)
		return
	}

	if len(resp) < 1 {
		err = errors.New("maps geo-coding response invalid")
		utils.LogErrorWithLevel(err, utils.LogError)
		return
	}

	location := resp[0].Geometry.Location
	lat = location.Lat
	lng = location.Lng

	cityName := resp[0].AddressComponents[0].LongName
	query.City = cityName

	return
}
