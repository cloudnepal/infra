package access

import (
	"fmt"
	"math"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/infrahq/infra/internal"
	"github.com/infrahq/infra/internal/logging"
	"github.com/infrahq/infra/internal/server/data"
	"github.com/infrahq/infra/internal/server/models"
)

func SetupRequired(c *gin.Context) (bool, error) {
	db, err := requireAuthorization(c)
	if err != nil {
		return false, err
	}

	settings, err := data.GetSettings(db)
	if err != nil {
		return false, err
	}

	return settings.SetupRequired, nil
}

func Setup(c *gin.Context) (string, *models.AccessKey, error) {
	db, err := requireAuthorization(c)
	if err != nil {
		return "", nil, err
	}

	settings, err := data.GetSettings(db)
	if err != nil {
		logging.S.Errorf("settings: %w", err)
		return "", nil, internal.ErrForbidden
	}

	if !settings.SetupRequired {
		return "", nil, internal.ErrForbidden
	}

	name := "admin"
	machine := &models.Machine{
		Name:        name,
		Description: "Infra admin machine identity",
		Permissions: string(PermissionAllInfra),
		LastSeenAt:  time.Now(),
	}

	if err := data.CreateMachine(db, machine); err != nil {
		return "", nil, err
	}

	key := &models.AccessKey{
		Name:      fmt.Sprintf("%s access key", name),
		IssuedFor: machine.PolymorphicIdentifier(),
		ExpiresAt: time.Now().Add(math.MaxInt64),
	}

	raw, err := data.CreateAccessKey(db, key)
	if err != nil {
		return "", nil, err
	}

	settings.SetupRequired = false
	if err := data.SaveSettings(db, settings); err != nil {
		return "", nil, err
	}

	return raw, key, nil
}