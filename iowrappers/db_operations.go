package iowrappers

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/user"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"golang.org/x/crypto/bcrypt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	UserCollection           = "User"
	PlanningEventsCollection = "PlanningEvents"
)

type DatabaseHandler interface {
	PlaceSearch(req *PlaceSearchRequest) ([]POI.Place, error)
}

type CollectionHandler interface {
	Search(radius uint, latitude float64, longitude float64) []POI.Place
}

// handles operations at database level
// DbHandler.handlers manages collection handlers
type DbHandler struct {
	dbName   string
	Session  *mgo.Session
	handlers map[string]*CollHandler
}

// handles operations at collection level
// each collection handler is associated with a collection in a database at Init
type CollHandler struct {
	session  *mgo.Session
	dbName   string
	collName string
}

type PlanningEvent struct {
	User      string `json:"user"`
	City      string `json:"city"`
	Country   string `json:"country"`
	Timestamp string `json:"timestamp"`
}

func (dbHandler *DbHandler) Init(DbName string, url string) {
	dbHandler.handlers = make(map[string]*CollHandler)
	dbHandler.dbName = DbName
	dbHandler.CreateSession(url)

	dbHandler.SetCollHandler(UserCollection)
	dbHandler.SetCollHandler(PlanningEventsCollection)
}

func (dbHandler DbHandler) CreatePlanningEvent(event PlanningEvent) {
	_ = dbHandler.handlers[PlanningEventsCollection].GetCollection().Insert(event)
}

func (dbHandler *DbHandler) CreateSession(uri string) {
	session, err := mgo.Dial(uri)
	utils.LogErrorWithLevel(err, utils.LogError)
	dbHandler.Session = session
}

func (dbHandler *DbHandler) SetCollHandler(collectionName string) {
	if _, exist := dbHandler.handlers[collectionName]; !exist {
		collHandler := &CollHandler{}
		dbHandler.handlers[collectionName] = collHandler
		collHandler.Init(dbHandler, dbHandler.dbName, collectionName)
	}
}

// create a user and persist in database
// username is primary key
func (dbHandler *DbHandler) CreateUser(user user.User) (err error) {
	psw, _ := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	user.Password = string(psw)
	err = dbHandler.handlers[UserCollection].GetCollection().Insert(user)
	return
}

// only admin user can remove users
// admin users cannot be removed
func (dbHandler *DbHandler) RemoveUser(currentUsername string, currentUserPassword string, username string) (err error) {
	currentUser, userFindErr := dbHandler.FindUser(currentUsername)
	if userFindErr != nil {
		err = userFindErr
		return
	}

	isAdmin := currentUser.UserLevel == user.LevelAdminString
	if !isAdmin {
		err = errors.New("operation forbidden, not an admin user")
		return
	}

	_, _, loginErr := dbHandler.UserLogin(user.Credential{
		Username: currentUsername,
		Password: currentUserPassword,
	}, false)
	if loginErr != nil {
		err = loginErr
		return
	}

	adminUsers := strings.Split(os.Getenv("ADMIN_USERS"), ",")
	for _, u := range adminUsers {
		if username == u {
			err = errors.New("operation forbidden, cannot remove admin user")
			return
		}
	}

	err = dbHandler.handlers[UserCollection].GetCollection().RemoveId(username)
	return
}

func (dbHandler *DbHandler) FindUser(username string) (u *user.User, err error) {
	u = &user.User{}
	err = dbHandler.handlers[UserCollection].GetCollection().FindId(username).One(&u)
	return
}

// user login is used when a new user that holds no JWT or an existing user with expired JWT
func (dbHandler *DbHandler) UserLogin(credential user.Credential, issueJWT bool) (token string, tokenExpirationTime time.Time, err error) {
	u, userFindErr := dbHandler.FindUser(credential.Username)
	if userFindErr != nil { // user not found
		err = errors.New("user not found")
		return
	}

	pswCompErr := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(credential.Password))
	if pswCompErr != nil { // wrong password
		err = errors.New("wrong password")
		return
	}

	lastLoginTime := time.Now() // UTC time
	_ = dbHandler.handlers[UserCollection].GetCollection().UpdateId(credential.Username,
		bson.M{"$set": bson.M{"last_login_time": lastLoginTime}})

	// issue JWT
	if issueJWT {
		tokenExpirationTime = lastLoginTime.Add(user.JWTExpirationTime)
		expiresAt := tokenExpirationTime.Unix() // expires after 10 days
		jwtSigningSecret := os.Getenv("JWT_SIGNING_SECRET")

		jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username":       u.Username,
			"StandardClaims": jwt.StandardClaims{ExpiresAt: expiresAt},
		})

		token, err = jwtToken.SignedString([]byte(jwtSigningSecret))
	}
	return
}

// This design make sure that explicit call to SetCollHandler have to be made for new collection creation.
// Prevent accidentally creating collections in PlaceSearch method.
// Since the nearby search in Redis has considered maximum search radius, in this method we only need to use the updated
// search radius to search one more time in database.
func (dbHandler *DbHandler) PlaceSearch(req *PlaceSearchRequest) (places []POI.Place, err error) {
	collName := string(req.PlaceCat)
	dbHandler.SetCollHandler(collName)
	err = EnsureSpatialIndex(dbHandler.handlers[collName].GetCollection())
	if err != nil {
		return
	}

	if _, exist := dbHandler.handlers[collName]; !exist {
		err = fmt.Errorf("collection %s does not exist", collName)
		return
	}

	collHandler := dbHandler.handlers[collName]

	latLng, _ := utils.ParseLocation(req.Location)
	lat := latLng[0]
	lng := latLng[1]

	searchRadius := req.Radius
	places = collHandler.Search(searchRadius, lat, lng)

	return
}

func (dbHandler *DbHandler) InsertPlace(place POI.Place, placeCat POI.PlaceCategory, wg *sync.WaitGroup, newDocCounter *uint64) {
	defer wg.Done()
	collName := string(placeCat)
	if _, exist := dbHandler.handlers[collName]; !exist {
		Logger.Error(fmt.Errorf("collection %s does not exist", collName))
		return
	}
	collHandler := dbHandler.handlers[collName]
	err := collHandler.InsertPlace(place)
	if err != nil {
		if mgo.IsDup(err) {
			Logger.Debugf("Database updating %s", place.Name)
			_ = collHandler.UpdatePlace(place)
		} else {
			Logger.Error(err)
		}
	} else {
		atomic.AddUint64(newDocCounter, 1)
	}
}

// ensure the 2d-sphere index exist
func EnsureSpatialIndex(coll *mgo.Collection) (err error) {
	index := mgo.Index{
		Key: []string{"$2dsphere:location"},
	}
	err = coll.EnsureIndex(index)
	return
}

func (collHandler *CollHandler) Init(dbHandler *DbHandler, databaseName string, collectionName string) {
	collHandler.session = dbHandler.Session
	collHandler.dbName = databaseName
	collHandler.collName = collectionName
}

func (collHandler *CollHandler) GetCollection() (coll *mgo.Collection) {
	coll = collHandler.session.DB(collHandler.dbName).C(collHandler.collName)
	return
}

func (collHandler *CollHandler) InsertPlace(place POI.Place) error {
	err := collHandler.GetCollection().Insert(place)
	return err
}

func (collHandler *CollHandler) UpdatePlace(place POI.Place) error {
	query := bson.M{"_id": place.ID}
	err := collHandler.GetCollection().Update(query, place)
	return err
}

// MongoDB geo-spatial search
// Need to create 2d sphere index before use, e.g. db.Eatery.createIndex({ location: "2dsphere" })
func (collHandler *CollHandler) Search(radius uint, latitude float64, longitude float64) (places []POI.Place) {
	query := bson.M{
		"location": bson.M{
			"$nearSphere": bson.M{
				"$geometry": bson.M{
					"type": "Point",
					// per MongoDB geoJSON requirements, specify the longitude first and then latitude
					"coordinates": [2]float64{longitude, latitude},
				},
				"$maxDistance": radius,
			},
		},
	}
	coll := collHandler.GetCollection()
	utils.LogErrorWithLevel(coll.Find(query).All(&places), utils.LogError)
	return
}
