package auth

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/clarateach/backend/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailExists        = errors.New("email already registered")
)

type contextKey string

const UserContextKey contextKey = "user"

// Claims represents JWT claims for user authentication
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// WorkspaceClaims represents JWT claims for workspace access
type WorkspaceClaims struct {
	WorkshopID string `json:"workshop_id"`
	Seat       int    `json:"seat"`
	jwt.RegisteredClaims
}

// GetJWTSecret returns the JWT secret from env or default
func GetJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "clarateach-dev-secret-change-in-production"
	}
	return []byte(secret)
}

// GetWorkspaceTokenSecret returns the workspace token secret from env or default
func GetWorkspaceTokenSecret() []byte {
	secret := os.Getenv("WORKSPACE_TOKEN_SECRET")
	if secret == "" {
		secret = "clarateach-workspace-dev-secret-change-in-production"
	}
	return []byte(secret)
}

// GenerateWorkspaceToken creates a JWT token for workspace access
func GenerateWorkspaceToken(workshopID string, seat int) (string, error) {
	claims := &WorkspaceClaims{
		WorkshopID: workshopID,
		Seat:       seat,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(4 * time.Hour)), // 4 hour expiry
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   workshopID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetWorkspaceTokenSecret())
}

// ValidateWorkspaceToken validates a workspace JWT token and returns the claims
func ValidateWorkspaceToken(tokenString string) (*WorkspaceClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &WorkspaceClaims{}, func(token *jwt.Token) (interface{}, error) {
		return GetWorkspaceTokenSecret(), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*WorkspaceClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid workspace token")
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares a password with a hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken creates a JWT token for a user
func GenerateToken(user *store.User) (string, error) {
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return GetJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// AuthMiddleware creates a middleware that validates JWT tokens
func AuthMiddleware(s store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization required", http.StatusUnauthorized)
				return
			}

			// Extract Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Validate token
			claims, err := ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Get user from database
			user, err := s.GetUser(claims.UserID)
			if err != nil || user == nil {
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}

			// Add user to context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminMiddleware ensures the user is an admin
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if user.Role != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext retrieves the user from context
func GetUserFromContext(ctx context.Context) *store.User {
	user, ok := ctx.Value(UserContextKey).(*store.User)
	if !ok {
		return nil
	}
	return user
}
