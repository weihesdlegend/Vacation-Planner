package POI

import (
	"log"
	"reflect"
	"regexp"

	"googlemaps.github.io/maps"
)

type Weekday uint8

const (
	DateMonday Weekday = iota
	DateTuesday
	DateWednesday
	DateThursday
	DateFriday
	DateSaturday
	DateSunday
)

type PlacePhoto struct {
	// reference from Google Images
	Reference string `bson:"reference"`
	// the maximum height of the image
	Height int `bson:"height"`
	// the maximum width of the image
	Width int `bson:"width"`
}

type BusinessStatus string

const (
	Operational        BusinessStatus = "OPERATIONAL"
	ClosedTemporarily  BusinessStatus = "CLOSED_TEMPORARILY"
	ClosedPermanently  BusinessStatus = "CLOSED_PERMANENTLY"
	StatusNotAvailable BusinessStatus = "STATUS_NOT_AVAILABLE"
)

type Place struct {
	ID               string         `bson:"_id"`
	Name             string         `bson:"name"`
	Status           BusinessStatus `bson:"business_status"`
	LocationType     LocationType   `bson:"location_type"`
	Address          Address        `bson:"address"`
	FormattedAddress string         `bson:"formatted_address"`
	Location         Location       `bson:"location"`
	PriceLevel       int            `bson:"price_level"`
	Rating           float32        `bson:"rating"`
	Hours            [7]string      `bson:"hours"`
	URL              string         `bson:"url"`
	Photo            PlacePhoto     `bson:"photo"`
	UserRatingsTotal int            `bson:"user_ratings_total"`
}

type Location struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

type Address struct {
	POBox        string
	ExtendedAddr string
	StreetAddr   string
	Locality     string
	Region       string
	PostalCode   string
	Country      string
}

func (place *Place) GetName() string {
	return place.Name
}

func (place *Place) GetType() LocationType {
	return place.LocationType
}

func (place *Place) GetStatus() BusinessStatus {
	return place.Status
}

func (place *Place) GetHour(day Weekday) string {
	return place.Hours[day]
}

func (place *Place) GetID() string {
	return place.ID
}

//Sample Address in adr micro-format
//665 3rd St.
//Suite 207
//San Francisco, CA 94107
//U.S.A.
func (place *Place) GetAddress() Address {
	return place.Address
}

func (place *Place) GetFormattedAddress() string {
	return place.FormattedAddress
}

func (place *Place) GetLocation() [2]float64 {
	return place.Location.Coordinates
}

func (place *Place) GetPriceLevel() int {
	return place.PriceLevel
}

func (place *Place) GetRating() float32 {
	return place.Rating
}

func (place *Place) GetURL() string {
	return place.URL
}

func (place *Place) GetUserRatingsTotal() int {
	return place.UserRatingsTotal
}

// Set name if POI name changed
func (place *Place) SetName(name string) {
	place.Name = name
}

func (place *Place) SetStatus(status string) {
	switch status {
	case "OPERATIONAL":
		place.Status = Operational
	case "CLOSED_TEMPORARILY":
		place.Status = ClosedTemporarily
	case "CLOSED_PERMANENTLY":
		place.Status = ClosedPermanently
	default:
		place.Status = StatusNotAvailable
	}
}

// Set human-readable Address of this place
func (place *Place) SetFormattedAddress(formattedAddress string) {
	place.FormattedAddress = formattedAddress
}

// Set type if POI type changed
func (place *Place) SetType(t LocationType) {
	place.LocationType = t
}

// Set time if POI opening hour changed for some day in a week
func (place *Place) SetHour(day Weekday, hour string) {
	switch day {
	case DateSunday:
		place.Hours[day] = hour
	case DateMonday:
		place.Hours[day] = hour
	case DateTuesday:
		place.Hours[day] = hour
	case DateWednesday:
		place.Hours[day] = hour
	case DateThursday:
		place.Hours[day] = hour
	case DateFriday:
		place.Hours[day] = hour
	case DateSaturday:
		place.Hours[day] = hour
	default:
		log.Fatalf("day specified (%d) is not in range of 0-6", day)
	}
}

func (place *Place) SetID(id string) {
	place.ID = id
}

func (place *Place) SetAddress(addr string) {
	if addr == "" {
		return
	}
	p := regexp.MustCompile(`<.*?>.*?<`)
	pVal := regexp.MustCompile(`>.*<`)
	pFieldName := regexp.MustCompile(`".*"`)
	fields := p.FindAllString(addr, -1)
	for _, field := range fields {
		fieldName := pFieldName.FindString(field)
		value := pVal.FindString(field)
		val := value[1 : len(value)-1]
		switch fieldName {
		case `"post-office-box"`:
			place.Address.POBox = val
		case `"extended-address"`:
			place.Address.ExtendedAddr = val
		case `"street-address"`:
			place.Address.StreetAddr = val
		case `"locality"`:
			place.Address.Locality = val
		case `"region"`:
			place.Address.Region = val
		case `"postal-code"`:
			place.Address.PostalCode = val
		case `"country-name"`:
			place.Address.Country = val
		}
	}
}

func (place *Place) SetLocation(location [2]float64) {
	place.Location.Coordinates = location
	place.Location.Type = "Point"
}

func (place *Place) SetPriceLevel(priceRange int) {
	place.PriceLevel = priceRange
}

func (place *Place) SetRating(rating float32) {
	place.Rating = rating
}

func (place *Place) SetURL(url string) {
	place.URL = url
}

func (place *Place) SetPhoto(photo *maps.Photo) {
	if val := reflect.ValueOf(photo); !val.IsNil() {
		place.Photo.Reference = photo.PhotoReference
		place.Photo.Height = photo.Height
		place.Photo.Width = photo.Width
	}
}

func (place *Place) SetUserRatingsTotal(userRatingsTotal int) {
	place.UserRatingsTotal = userRatingsTotal
}
