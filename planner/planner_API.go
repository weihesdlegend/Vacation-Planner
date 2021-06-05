package planner

import (
	"context"
	"errors"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/iowrappers"
	"github.com/weihesdlegend/Vacation-planner/matching"
	"github.com/weihesdlegend/Vacation-planner/solution"
	"github.com/weihesdlegend/Vacation-planner/user"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MaxPlacesPerSlot   = 4
	MaxPlacesPerDay    = 12
	ServerTimeout      = time.Second * 15
	jobQueueBufferSize = 1000
)

type MyPlanner struct {
	RedisClient        iowrappers.RedisClient
	RedisStreamName    string
	Solver             solution.Solver
	ResultHTMLTemplate *template.Template
	PlanningEvents     chan iowrappers.PlanningEvent
	Environment        string
	Configs            map[string]interface{}
}

type TimeSectionPlace struct {
	PlaceName string   `json:"place_name"`
	StartTime POI.Hour `json:"start_time"`
	EndTime   POI.Hour `json:"end_time"`
	Address   string   `json:"address"`
	URL       string   `json:"url"`
}

type TimeSectionPlaces struct {
	Places []TimeSectionPlace `json:"places"`
}

type PlanningResponse struct {
	TravelDestination string                `json:"travel_destination"`
	Places            [][]TimeSectionPlaces `json:"time_section_places"`
	Err               error                 `json:"error"`
	StatusCode        uint                  `json:"status_code"`
}

// validate REST API input
func validateSearchRadius(searchRadius string) bool {
	searchRadiusPattern := "^[1-9][0-9]{2,5}$" // limit range to 100 -- 99999
	if matched, _ := regexp.Match(searchRadiusPattern, []byte(searchRadius)); !matched {
		return false
	}
	return true
}

type PlanningPostRequest struct {
	Country   string      `json:"country"`
	City      string      `json:"city"`
	Weekday   POI.Weekday `json:"weekday"`
	StartTime POI.Hour    `json:"start_time"`
	EndTime   POI.Hour    `json:"end_time"`
	NumVisit  uint        `json:"num_visit"`
	NumEatery uint        `json:"num_eatery"`
}

func (planner *MyPlanner) Init(mapsClientApiKey string, redisURL *url.URL, redisStreamName string, configs map[string]interface{}) {
	planner.PlanningEvents = make(chan iowrappers.PlanningEvent, jobQueueBufferSize)
	planner.RedisClient = iowrappers.CreateRedisClient(redisURL)
	planner.RedisStreamName = redisStreamName
	if redisStreamName == "" {
		planner.RedisStreamName = "stream:planning_api_usage"
	}

	PoiSearcher := iowrappers.CreatePoiSearcher(mapsClientApiKey, redisURL)

	planner.Solver.Init(PoiSearcher)

	planner.ResultHTMLTemplate = template.Must(template.ParseFiles("templates/plan_layout.html"))
	planner.Environment = strings.ToLower(os.Getenv("ENVIRONMENT"))
	planner.Configs = configs
	if v, exists := planner.Configs["server:google_maps:detailed_search_fields"]; exists {
		planner.Solver.Matcher.PoiSearcher.GetMapsClient().SetDetailedSearchFields(v.([]string))
	}
}

func (planner *MyPlanner) SingleDayNearbySearchHandler(context *gin.Context) {
	country := context.DefaultQuery("country", "USA")
	city := context.DefaultQuery("city", "San Diego")
	radius := context.DefaultQuery("radius", "10000")
	weekday := context.DefaultQuery("weekday", "5") // Saturday
	category := strings.ToLower(context.DefaultQuery("category", "visit"))

	weekdayUint, weekdayParsingErr := strconv.ParseUint(weekday, 10, 8)
	if weekdayParsingErr != nil || weekdayUint > 6 {
		context.String(http.StatusBadRequest, "invalid weekday of %d", weekdayUint)
		return
	}
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)

	var placeCategory POI.PlaceCategory
	switch category {
	case "visit":
		placeCategory = POI.PlaceCategoryVisit
	case "eatery":
		placeCategory = POI.PlaceCategoryEatery
	}

	location := strings.Join([]string{city, country}, ",")
	places, err := solution.NearbySearchWithPlaceView(context, planner.Solver.Matcher, location, POI.Weekday(weekdayUint), uint(searchRadius_), matching.TimeSlot{Slot: POI.TimeInterval{
		Start: 8,
		End:   21,
	}}, placeCategory)
	if err != nil {
		context.JSON(http.StatusInternalServerError, "sorry please try later")
		return
	}
	context.JSON(http.StatusOK, gin.H{"places": places})
}
func (planner *MyPlanner) SingleDayTimeCostPlanHandler(ctx *gin.Context) {
	requestId := requestid.Get(ctx)
	country := ctx.DefaultQuery("country", "USA")
	city := ctx.DefaultQuery("city", "San Diego")
	radius := ctx.DefaultQuery("radius", "10000")
	weekday := ctx.DefaultQuery("weekday", "5") // Saturday
	budget := ctx.DefaultQuery("budget", "1500")
	starthourstr := ctx.DefaultQuery("starthour", "8")
	endhourstr := ctx.DefaultQuery("endhour", "21")
	startHour, _ := strconv.ParseUint(starthourstr, 10, 8)
	endHour, _ := strconv.ParseUint(endhourstr, 10, 8)
	weekdayUint, weekdayParsingErr := strconv.ParseUint(weekday, 10, 8)
	if weekdayParsingErr != nil || weekdayUint < 0 || weekdayUint > 6 {
		ctx.String(http.StatusBadRequest, "invalid weekday of %d", weekdayUint)
		return
	}
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)
	location := strings.Join([]string{city, country}, ",")
	budgetUint, budgetParsingErr := strconv.ParseUint(budget, 10, 32)
	if budgetParsingErr != nil {
		ctx.String(http.StatusBadRequest, "invalid input of budget %s", budget)
		return
	}
	c := context.WithValue(ctx, "request_id", requestId)
	places, err := solution.NearbySearchAllCategories(c, planner.Solver.Matcher, location, POI.Weekday(weekdayUint), uint(searchRadius_), matching.TimeSlot{Slot: POI.TimeInterval{
		Start: POI.Hour(startHour),
		End:   POI.Hour(endHour),
	}})
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	knapsackInterval := matching.QueryTimeInterval{
		Day: POI.Weekday(weekdayUint),
		StartHour: uint8(startHour),
		EndHour: uint8(endHour),
	}

	//do knapsack
	result, totalCost, totalTimeSpent:= matching.Knapsack(places, knapsackInterval, uint(budgetUint))
	if result == nil {
		ctx.JSON(http.StatusInternalServerError, "no plan could be provided")
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"Time Budget Plan": result, "Cost":totalCost, "Time":totalTimeSpent})
}
func (planner *MyPlanner) Destroy() {
	iowrappers.DestroyLogger()
	planner.RedisClient.Destroy()
}

func (planner *MyPlanner) ReverseGeocodingHandler(context *gin.Context) {
	latitude, _ := strconv.ParseFloat(context.Query("lat"), 64)
	longitude, _ := strconv.ParseFloat(context.Query("lng"), 64)
	result, err := planner.Solver.Matcher.PoiSearcher.GetMapsClient().ReverseGeocoding(context, latitude, longitude)
	if err != nil {
		log.Error(err)
		context.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	context.JSON(http.StatusOK, gin.H{
		"results": result,
	})
}

func (planner *MyPlanner) UserRatingsTotalMigrationHandler(context *gin.Context) {
	_, authenticationErr := planner.UserAuthentication(context, context.Request, user.LevelAdmin)
	if authenticationErr != nil {
		context.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	if err := planner.Solver.Matcher.PoiSearcher.AddUserRatingsTotal(context.Request.Context()); err != nil {
		log.Error(err)
	}
}

func (planner *MyPlanner) UrlMigrationHandler(context *gin.Context) {
	_, authenticationErr := planner.UserAuthentication(context, context.Request, user.LevelAdmin)
	if authenticationErr != nil {
		context.JSON(http.StatusUnauthorized, gin.H{"error": authenticationErr.Error()})
		return
	}
	if err := planner.Solver.Matcher.PoiSearcher.AddUrl(context.Request.Context()); err != nil {
		log.Error(err)
	}
}

func (planner *MyPlanner) PlaceStatsHandler(context *gin.Context) {
	var placeCount int
	var err error
	if _, placeCount, err = planner.RedisClient.GetPlaceCountInRedis(context); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var eateryCount int64
	if eateryCount, err = planner.RedisClient.GetPlaceCountByCategory(context, POI.PlaceCategoryEatery); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var visitCount int64
	if visitCount, err = planner.RedisClient.GetPlaceCountByCategory(context, POI.PlaceCategoryVisit); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	context.JSON(http.StatusOK, gin.H{
		"place count":  placeCount,
		"eatery count": eateryCount,
		"visit count":  visitCount,
	})
}

type GeocodeCityView struct {
	Count  int
	Cities map[string]string
}

func (planner *MyPlanner) CityStatsHandler(context *gin.Context) {
	var cityCount int
	var err error
	var geocodes map[string]string

	if geocodes, err = planner.RedisClient.GetCityCountInRedis(context); err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cityCount = len(geocodes)
	view := GeocodeCityView{
		Count:  cityCount,
		Cities: geocodes,
	}
	context.JSON(http.StatusOK, view)
}

// Planning solves the single-day, single-city planning task
func (planner *MyPlanner) Planning(ctx context.Context, planningRequest *solution.PlanningRequest, user string) (resp PlanningResponse) {
	var planningResponse solution.PlanningResponse

	planner.Solver.Solve(ctx, planner.RedisClient, planningRequest, &planningResponse)

	if planningResponse.Err != nil {
		resp.Err = planningResponse.Err
		resp.StatusCode = planningResponse.ErrorCode
		return
	}

	// logging planning API usage for valid requests
	if len(planningRequest.SlotRequests) > 0 {
		countryCity := planningRequest.SlotRequests[0].Location
		countryAndCity := strings.Split(countryCity, ",")
		event := iowrappers.PlanningEvent{
			User:      user,
			Country:   countryAndCity[1],
			City:      countryAndCity[0],
			Timestamp: time.Now().Format(time.RFC3339),
		}
		planner.PlanningEvents <- event
		planner.PlanningEventLogging(event)
	}

	if len(planningResponse.Solutions) == 0 {
		resp.Err = errors.New("cannot find a valid solution")
		resp.StatusCode = solution.NoValidSolution
		return
	}

	topSolutions := planningResponse.Solutions
	resp.Places = make([][]TimeSectionPlaces, len(topSolutions))
	for sIdx, topSolution := range topSolutions {
		for idx, slotSol := range topSolution.SlotSolutions {
			timeSectionPlaces := TimeSectionPlaces{
				Places: make([]TimeSectionPlace, 0),
			}
			for pIdx, placeName := range slotSol.PlaceNames {
				timeSectionPlaces.Places = append(timeSectionPlaces.Places, TimeSectionPlace{
					PlaceName: placeName,
					StartTime: planningRequest.SlotRequests[idx].StayTimes[pIdx].Slot.Start,
					EndTime:   planningRequest.SlotRequests[idx].StayTimes[pIdx].Slot.End,
					Address:   slotSol.PlaceAddresses[pIdx],
					URL:       slotSol.PlaceURLs[pIdx],
				})
			}
			resp.Places[sIdx] = append(resp.Places[sIdx], timeSectionPlaces)
		}
	}

	resp.StatusCode = solution.ValidSolutionFound
	if len(planningRequest.SlotRequests) > 0 {
		resp.TravelDestination = strings.Title(strings.Split(planningRequest.SlotRequests[0].Location, ",")[0])
	} else {
		resp.TravelDestination = "Dream Vacation Destination"
	}
	return
}

// API definitions
func (planner *MyPlanner) indexPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{})
}

// HTTP POST API end-point
func (planner *MyPlanner) postPlanningApi(context *gin.Context) {
	var username = "guest" // default username
	if planner.Environment == "production" {
		var authenticationErr error
		username, authenticationErr = planner.UserAuthentication(context, context.Request, user.LevelRegular)
		if authenticationErr != nil {
			utils.LogErrorWithLevel(authenticationErr, utils.LogDebug)
			planner.login(context)
			return
		}
	}

	req := PlanningPostRequest{}
	err := context.ShouldBindJSON(&req)
	utils.LogErrorWithLevel(err, utils.LogInfo)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	planningReq, err := processPlanningPostRequest(&req)
	utils.LogErrorWithLevel(err, utils.LogInfo)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	planningResp := planner.Planning(context, &planningReq, username)
	if planningResp.Err != nil && planningResp.StatusCode == http.StatusNotFound {
		context.JSON(http.StatusNotFound, gin.H{"error": "No solution is found"})
		return
	}
	// generate valid solution
	utils.LogErrorWithLevel(planner.ResultHTMLTemplate.Execute(context.Writer, planningResp), utils.LogError)
}

// HTTP GET API end-point
// Return top planning result to user
func (planner *MyPlanner) getPlanningApi(ctx *gin.Context) {
	var username = "guest" // default username
	if strings.ToLower(planner.Environment) == "production" {
		var authenticationErr error
		username, authenticationErr = planner.UserAuthentication(ctx, ctx.Request, user.LevelRegular)
		if authenticationErr != nil {
			utils.LogErrorWithLevel(authenticationErr, utils.LogDebug)
			planner.login(ctx)
			return
		}
	}

	requestId := requestid.Get(ctx)
	country := ctx.DefaultQuery("country", "USA")
	city := ctx.DefaultQuery("city", "San Diego")
	radius := ctx.DefaultQuery("radius", "10000")
	weekday := ctx.DefaultQuery("weekday", "5") // Saturday
	numResults := ctx.DefaultQuery("numberResults", "5")

	numResultsInt, numResultsParsingErr := strconv.ParseUint(numResults, 10, 64)
	if numResultsParsingErr != nil {
		ctx.String(http.StatusBadRequest, "number of planning results of %d is invalid", numResultsInt)
		return
	}
	iowrappers.Logger.Debugf("[%s] number of requested planning results is %s", requestId, numResults)

	weekdayUint, weekdayParsingErr := strconv.ParseUint(weekday, 10, 8)
	if weekdayParsingErr != nil || weekdayUint > 6 {
		ctx.String(http.StatusBadRequest, "invalid weekday of %d", weekdayUint)
		return
	}

	if !validateSearchRadius(radius) {
		ctx.String(http.StatusBadRequest, "invalid search radius of %s", radius)
		return
	}

	cityCountry := city + "," + country

	planningReq := solution.GetStandardRequest(POI.Weekday(weekdayUint), numResultsInt)
	searchRadius_, _ := strconv.ParseUint(radius, 10, 32)
	planningReq.SearchRadius = uint(searchRadius_)

	for slotReqIdx := range planningReq.SlotRequests {
		planningReq.SlotRequests[slotReqIdx].Location = cityCountry // set to the same location from URL
	}

	c := context.WithValue(ctx, "request_id", requestId)
	planningResp := planner.Planning(c, &planningReq, username)

	err := planningResp.Err
	if err != nil {
		if planningResp.StatusCode == solution.InvalidRequestLocation {
			ctx.String(http.StatusBadRequest, err.Error())
		} else if planningResp.StatusCode == solution.NoValidSolution {
			errString := "No valid solution is found.\n Please try to search with larger radius."
			ctx.String(http.StatusBadRequest, errString)
		}
		return
	}

	utils.LogErrorWithLevel(planner.ResultHTMLTemplate.Execute(ctx.Writer, planningResp), utils.LogError)
}

func (planner *MyPlanner) login(c *gin.Context) {
	c.HTML(http.StatusOK, "login_page.html", gin.H{})
}

func (planner *MyPlanner) signup(c *gin.Context) {
	c.HTML(http.StatusOK, "signup_page.html", gin.H{})
}

func (planner MyPlanner) SetupRouter(serverPort string) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	if planner.Environment == "debug" {
		gin.SetMode(gin.DebugMode)
	}
	gin.DefaultWriter = ioutil.Discard

	myRouter := gin.Default()
	myRouter.LoadHTMLGlob("templates/*")
	// trace ID
	myRouter.Use(requestid.New())

	// cors settings
	// TODO: change to front-end domain once front-end server is deployed
	myRouter.Use(cors.Default())

	myRouter.GET("", planner.indexPageHandler)

	v1 := myRouter.Group("/v1")
	{
		v1.GET("/plans", planner.getPlanningApi)
		v1.POST("/plans", planner.postPlanningApi)
		v1.POST("/signup", planner.UserSignup)
		v1.POST("/login", planner.UserLogin)
		v1.GET("/reverse-geocoding", planner.ReverseGeocodingHandler)
		v1.GET("/single-day-nearby-search", planner.SingleDayNearbySearchHandler)
		v1.GET("/log-in", planner.login)
		v1.GET("/sign-up", planner.signup)
		v1.GET("/time-budget-plan", planner.SingleDayTimeCostPlanHandler)
		migrations := v1.Group("/migrate")
		{
			migrations.GET("/user-ratings-total", planner.UserRatingsTotalMigrationHandler)
			migrations.GET("/url", planner.UrlMigrationHandler)
		}
	}

	// API endpoints for collecting database statistics
	stats := myRouter.Group("/stats")
	{
		stats.GET("places", planner.PlaceStatsHandler)
		stats.GET("cities", planner.CityStatsHandler)
	}

	svr := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      myRouter,
		ReadTimeout:  ServerTimeout,
		WriteTimeout: ServerTimeout,
	}

	return svr
}
