package planner

import (
	"context"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/weihesdlegend/Vacation-planner/user"
	"net/http"
	"os"
	"strings"
)

type UserLoginResponse struct {
	Username string `json:"username"`
	Jwt      string `json:"jwt"`
	Status   string `json:"status"`
}

// UserSignup handles user signup POST requests
// user submit username/password/email and user is created
// return bad request if creation fails
func (planner MyPlanner) UserSignup(context *gin.Context) {
	u := user.User{}

	decodeErr := context.ShouldBindJSON(&u)
	if decodeErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
		return
	}

	userLevel := user.LevelRegularString
	adminUsers := strings.Split(os.Getenv("ADMIN_USERS"), ",")
	for _, username := range adminUsers {
		if u.Username == username {
			userLevel = user.LevelAdminString
		}
	}

	u.UserLevel = userLevel

	createErr := planner.RedisClient.CreateUser(context, u)
	if createErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": createErr.Error()})
		return
	}
	context.JSON(http.StatusCreated, gin.H{"user creation success": u.Username})
}

// UserLogin handles login POST requests
// user submit credentials and return JWT if login is successful
func (planner MyPlanner) UserLogin(context *gin.Context) {
	c := user.Credential{}

	decodeErr := context.ShouldBindJSON(&c)
	if decodeErr != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": decodeErr.Error()})
		return
	}

	token, tokenExpirationTime, loginErr := planner.RedisClient.Authenticate(context, c)
	if loginErr != nil {
		log.Debug(loginErr)
		context.JSON(http.StatusUnauthorized, UserLoginResponse{
			Username: c.Username,
			Jwt:      "",
			Status:   "unauthorized",
		})
		return
	}

	http.SetCookie(context.Writer, &http.Cookie{
		Name:    "JWT",
		Value:   token,
		Expires: tokenExpirationTime,
	})

	http.SetCookie(context.Writer, &http.Cookie{
		Name:  "Username",
		Value: c.Username,
	})

	context.JSON(http.StatusOK, UserLoginResponse{
		Username: c.Username,
		Jwt:      token,
		Status:   "you are logged in",
	})
}

// UserAuthentication is an internal method for API to authenticate users at different levels
func (planner MyPlanner) UserAuthentication(context context.Context, r *http.Request, minimumUserLevel user.Level) (username string, err error) {
	cookie, cookieErr := r.Cookie("JWT")
	if cookieErr != nil {
		return "", cookieErr
	}

	jwtKey := []byte(os.Getenv("JWT_SIGNING_SECRET"))
	token, tokenErr := jwt.Parse(cookie.Value, func(tkn *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if tokenErr != nil {
		return "", tokenErr
	}

	if !token.Valid {
		return "", errors.New("invalid token")
	}

	userCookie, _ := r.Cookie("Username")
	username = userCookie.Value
	log.Debugf("the current user is %s", username)

	userFound, findUserErr := planner.RedisClient.FindUser(context, username)
	if findUserErr != nil {
		err = findUserErr
		return
	}
	var userLevel user.Level
	switch userFound.UserLevel {
	case user.LevelRegularString:
		userLevel = user.LevelRegular
	case user.LevelAdminString:
		userLevel = user.LevelAdmin
	}
	if userLevel < minimumUserLevel {
		log.Debugf("user level is %d, required %d", userLevel, minimumUserLevel)
		return username, errors.New("does not meet minimum user level requirement")
	}
	return username, nil
}
