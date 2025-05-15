package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/ClearThree/gophermart-bonus/internal/app/repositories"
	"golang.org/x/crypto/argon2"
	"strings"
)

type Argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

const passwordStringDelimiter = "$"

var argon2Params = Argon2Params{
	memory:      64 * 1024,
	iterations:  3,
	parallelism: 2,
	saltLength:  16,
	keyLength:   32,
}

var ErrInvalidHash = errors.New("the encoded hash is not in the correct format")
var ErrIncompatibleVersion = errors.New("incompatible version of argon2")
var ErrPasswordIsIncorrect = errors.New("provided password is incorrect")

type UserServiceInterface interface {
	Register(ctx context.Context, login string, password string) (uint64, error)
	Authenticate(ctx context.Context, login string, password string) (uint64, error)
	GetBalances(ctx context.Context, userID uint64) (float32, float32, error)
}

type UserService struct {
	userRepository repositories.UserRepositoryInterface
}

func NewUserService(userRepo repositories.UserRepositoryInterface) *UserService {
	return &UserService{userRepository: userRepo}
}

func (u UserService) Register(ctx context.Context, login string, password string) (uint64, error) {
	salt, err := u.generateSalt(argon2Params.saltLength)
	if err != nil {
		return 0, err
	}
	password, err = u.generateEncodedPasswordHash(password, salt, &argon2Params)
	if err != nil {
		return 0, err
	}
	user, err := u.userRepository.Create(ctx, login, password)
	if err != nil {
		return 0, err
	}
	logger.Log.Debugf("User created: %s", login)
	return user.ID, nil
}

func (u UserService) Authenticate(ctx context.Context, login string, password string) (uint64, error) {
	user, err := u.userRepository.Read(ctx, login)
	if err != nil {
		return 0, err
	}
	equal, err := u.comparePasswordAndHash(password, user.Password)
	if err != nil {
		return 0, err
	}
	if !equal {
		return 0, ErrPasswordIsIncorrect
	}
	return user.ID, nil
}

func (u UserService) GetBalances(ctx context.Context, userID uint64) (float32, float32, error) {
	balance, withdrawnBalances, err := u.userRepository.GetBalances(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	return balance, withdrawnBalances, nil
}

func (u UserService) generateEncodedPasswordHash(
	password string, salt []byte, argon2Params *Argon2Params) (string, error) {
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argon2Params.iterations,
		argon2Params.memory,
		argon2Params.parallelism,
		argon2Params.keyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	encodedHash := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Params.memory,
		argon2Params.iterations,
		argon2Params.parallelism,
		b64Salt,
		b64Hash)

	return encodedHash, nil
}

func (u UserService) decodeHash(encodedHash string) (*Argon2Params, []byte, []byte, error) {
	vals := strings.Split(encodedHash, passwordStringDelimiter)
	if len(vals) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}
	var version int
	_, err := fmt.Sscanf(vals[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, err
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}
	params := &Argon2Params{}
	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &params.memory, &params.iterations, &params.parallelism)
	if err != nil {
		return nil, nil, nil, err
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, err
	}
	params.saltLength = uint32(len(salt))
	hash, err := base64.RawStdEncoding.Strict().DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, err
	}
	params.keyLength = uint32(len(hash))
	return params, salt, hash, nil
}

func (u UserService) comparePasswordAndHash(password, encodedHash string) (bool, error) {
	params, salt, hash, err := u.decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	providedPasswordHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLength)

	if subtle.ConstantTimeCompare(hash, providedPasswordHash) == 1 {
		return true, nil
	}
	return false, nil

}

func (u UserService) generateSalt(length uint32) ([]byte, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
