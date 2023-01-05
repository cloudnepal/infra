package access

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"unicode"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/infrahq/infra/internal"
	"github.com/infrahq/infra/internal/generate"
	"github.com/infrahq/infra/internal/logging"
	"github.com/infrahq/infra/internal/server/data"
	"github.com/infrahq/infra/internal/server/models"
	"github.com/infrahq/infra/internal/validate"
)

func CreateCredential(c *gin.Context, user models.Identity) (string, error) {
	db, err := RequireInfraRole(c, models.InfraAdminRole)
	if err != nil {
		return "", HandleAuthErr(err, "user", "create", models.InfraAdminRole)
	}

	tmpPassword, err := generate.CryptoRandom(12, generate.CharsetPassword)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(tmpPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash: %w", err)
	}

	userCredential := &models.Credential{
		IdentityID:      user.ID,
		PasswordHash:    hash,
		OneTimePassword: true,
	}

	if err := data.CreateCredential(db, userCredential); err != nil {
		return "", err
	}

	_, err = data.CreateProviderUser(db, data.InfraProvider(db), &user)
	if err != nil {
		return "", fmt.Errorf("creating provider user: %w", err)
	}

	return tmpPassword, nil
}

func UpdateCredential(c *gin.Context, user *models.Identity, oldPassword, newPassword string) error {
	rCtx := GetRequestContext(c)
	isSelf := isIdentitySelf(rCtx, data.GetIdentityOptions{ByID: user.ID})

	// anyone can update their own credentials, so check authorization when not self
	if !isSelf {
		err := IsAuthorized(rCtx, models.InfraAdminRole)
		if err != nil {
			return HandleAuthErr(err, "user", "update", models.InfraAdminRole)
		}
	}

	// Users have to supply their old password to change their existing password
	if isSelf {
		if oldPassword == "" {
			errs := make(validate.Error)
			errs["oldPassword"] = append(errs["oldPassword"], "is required")
			return errs
		}

		userCredential, err := data.GetCredentialByUserID(rCtx.DBTxn, user.ID)
		if err != nil {
			return fmt.Errorf("existing credential: %w", err)
		}

		// compare the stored hash of the user's password and the hash of the presented password
		err = bcrypt.CompareHashAndPassword(userCredential.PasswordHash, []byte(oldPassword))
		if err != nil {
			// this probably means the password was wrong
			logging.L.Trace().Err(err).Msg("bcrypt comparison with oldpassword/newpassword failed")

			errs := make(validate.Error)
			errs["oldPassword"] = append(errs["oldPassword"], "invalid oldPassword")
			return errs
		}

	}

	if err := updateCredential(c, user, newPassword, isSelf); err != nil {
		return err
	}

	if !isSelf {
		// if the request is from an admin, the infra user may not exist yet, so create the
		// provider_user if it's missing.
		_, _ = data.CreateProviderUser(rCtx.DBTxn, data.InfraProvider(rCtx.DBTxn), user)
	}

	return nil
}

func updateCredential(c *gin.Context, user *models.Identity, newPassword string, isSelf bool) error {
	rCtx := GetRequestContext(c)
	db := rCtx.DBTxn

	if err := checkPasswordRequirements(db, newPassword); err != nil {
		return err
	}

	if err := checkBadPasswords(newPassword); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}

	userCredential, err := data.GetCredentialByUserID(db, user.ID)
	if err != nil {
		if errors.Is(err, internal.ErrNotFound) && !isSelf {
			if err := data.CreateCredential(db, &models.Credential{
				IdentityID:      user.ID,
				PasswordHash:    hash,
				OneTimePassword: true,
			}); err != nil {
				return fmt.Errorf("creating credentials: %w", err)
			}
			return nil
		}
		return fmt.Errorf("existing credential: %w", err)
	}

	userCredential.PasswordHash = hash
	userCredential.OneTimePassword = !isSelf

	if err := data.UpdateCredential(db, userCredential); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	if isSelf {
		// if we updated our own password, remove the password-reset scope from our access key.
		if accessKey := rCtx.Authenticated.AccessKey; accessKey != nil {
			accessKey.Scopes = sliceWithoutElement(accessKey.Scopes, models.ScopePasswordReset)
			if err = data.UpdateAccessKey(db, accessKey); err != nil {
				return fmt.Errorf("updating access key: %w", err)
			}
		}
	}
	return nil
}

func sliceWithoutElement(s []string, without string) []string {
	result := []string{}
	for _, v := range s {
		if v != without {
			result = append(result, v)
		}
	}
	return result
}

func GetRequestContext(c *gin.Context) RequestContext {
	if raw, ok := c.Get(RequestContextKey); ok {
		if rCtx, ok := raw.(RequestContext); ok {
			return rCtx
		}
	}
	return RequestContext{}
}

// list of special charaters from OWASP:
// https://owasp.org/www-community/password-special-characters
func isSymbol(r rune) bool {
	return (r >= '\u0020' && r <= '\u002F') || (r >= '\u003A' && r <= '\u0040') || (r >= '\u005B' && r <= '\u0060') || (r >= '\u007B' && r <= '\u007E')
}

func hasMinimumCount(password string, min int, minCheck func(rune) bool) bool {
	var count int
	for _, r := range password {
		if minCheck(r) {
			count++
		}
	}
	return count >= min
}

func checkPasswordRequirements(db data.ReadTxn, password string) error {
	settings, err := data.GetSettings(db)
	if err != nil {
		return err
	}

	requirements := []struct {
		minCount      int
		countFunc     func(rune) bool
		singularError string
		pluralError   string
	}{
		{settings.LengthMin, func(r rune) bool { return true }, "%d character", "%d characters"},
		{settings.LowercaseMin, unicode.IsLower, "%d lowercase letter", "%d lowercase letters"},
		{settings.UppercaseMin, unicode.IsUpper, "%d uppercase letter", "%d uppercase letters"},
		{settings.NumberMin, unicode.IsDigit, "%d number", "%d numbers"},
		{settings.SymbolMin, isSymbol, "%d symbol", "%d symbols"},
	}

	requirementError := make([]string, 0)

	valid := true
	for _, r := range requirements {
		if !hasMinimumCount(password, r.minCount, r.countFunc) {
			valid = false
		}

		switch {
		case r.minCount == 1:
			requirementError = append(requirementError, fmt.Sprintf(r.singularError, r.minCount))
		case r.minCount > 1:
			requirementError = append(requirementError, fmt.Sprintf(r.pluralError, r.minCount))
		}
	}

	if !valid {
		return validate.Error{"password": requirementError}
	}

	return nil
}

// checkBadPasswords checks if the password is a known bad password, i.e. a widely reused password.
func checkBadPasswords(password string) error {
	badPasswordsFile := os.Getenv("INFRA_SERVER_BAD_PASSWORDS_FILE")
	if badPasswordsFile == "" {
		return nil
	}

	file, err := os.Open(badPasswordsFile)
	if err != nil {
		return err
	}

	scan := bufio.NewScanner(file)
	scan.Split(bufio.ScanLines)
	for scan.Scan() {
		if scan.Text() == password {
			return fmt.Errorf("%w: cannot use a common password", internal.ErrBadRequest)
		}
	}

	if err := file.Close(); err != nil {
		return err
	}

	return nil
}
