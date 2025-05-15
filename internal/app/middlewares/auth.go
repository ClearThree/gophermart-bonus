package middlewares

import (
	"context"
	"errors"
	"github.com/ClearThree/gophermart-bonus/internal/app/config"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/golang-jwt/jwt/v4"
	"net/http"
	"strconv"
	"time"
)

type UserIDKeyType string

const AuthCookieName = "auth"
const UserIDKey UserIDKeyType = "UserID"

var ErrWrongAlgorithm = errors.New("unexpected signing method")
var ErrTokenIsNotValid = errors.New("invalid token passed")

type Claims struct {
	jwt.RegisteredClaims
	UserID uint64 `json:"user_id"`
}

func GenerateJWTString(userID uint64) (string, error) {
	if userID == 0 {
		return "", errors.New("invalid user id")
	}
	issueTime := time.Now()
	expireTime := issueTime.Add(time.Hour * time.Duration(config.Settings.JWTExpireHours))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "clearthree",
			IssuedAt:  jwt.NewNumericDate(issueTime),
			ExpiresAt: jwt.NewNumericDate(expireTime),
		},
		UserID: userID,
	})

	tokenString, err := token.SignedString([]byte(config.Settings.SecretKey))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func GetUserID(tokenString string) (uint64, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				logger.Log.Warnf("unexpected signing method: %v", t.Header["alg"])
				return nil, ErrWrongAlgorithm
			}
			return []byte(config.Settings.SecretKey), nil
		})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return claims.UserID, err
		}
		return 0, err
	}

	if !token.Valid {
		logger.Log.Info("Token is not valid")
		return 0, ErrTokenIsNotValid
	}

	return claims.UserID, nil
}

func AuthMiddleware(next http.Handler) http.Handler {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		var ctx = request.Context()
		token, err := request.Cookie(AuthCookieName)
		if err != nil {
			logger.Log.Warnf("No auth cookie")
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		userID, tokenErr := GetUserID(token.Value)
		if tokenErr != nil {
			logger.Log.Error(tokenErr)
			http.Error(writer, tokenErr.Error(), http.StatusUnauthorized)
			return
		}
		if userID == 0 {
			http.Error(writer, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx = context.WithValue(ctx, UserIDKey, userID)

		next.ServeHTTP(writer, request.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

type SetAuthWriter struct {
	writer http.ResponseWriter
}

func NewSetAuthWriter(writer http.ResponseWriter) *SetAuthWriter {
	return &SetAuthWriter{
		writer: writer,
	}
}

func (c *SetAuthWriter) Header() http.Header {
	return c.writer.Header()
}

func (c *SetAuthWriter) Write(p []byte) (int, error) {
	return c.writer.Write(p)
}

func (c *SetAuthWriter) WriteHeader(statusCode int) {
	if userIDFromHeader := c.Header().Get(string(UserIDKey)); userIDFromHeader != "" {
		userID, err := strconv.ParseUint(userIDFromHeader, 10, 64)
		if err != nil {
			logger.Log.Error(err)
			http.Error(c.writer, err.Error(), http.StatusInternalServerError)
		}
		JWTString, genErr := GenerateJWTString(userID)
		if genErr != nil {
			http.Error(c.writer, genErr.Error(), http.StatusInternalServerError)
			return
		}
		http.SetCookie(c.writer, &http.Cookie{
			Name:  AuthCookieName,
			Value: JWTString,
			Path:  "/",
		})
	}
	c.writer.WriteHeader(statusCode)
}

func SetAuthMiddleware(next http.Handler) http.Handler {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		writer = NewSetAuthWriter(writer)
		next.ServeHTTP(writer, request)
	}
	return http.HandlerFunc(fn)
}
