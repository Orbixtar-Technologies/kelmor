package phases

import (
	"context"
	"os"
	"time"

	"github.com/hosting-panel/panel/internal/auth"
	"github.com/hosting-panel/panel/internal/store"
)

func updateLiveAdminPassword(password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	dsn := os.Getenv("PANEL_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres:///panel_control?host=/var/run/postgresql"
	}
	pg, err := store.OpenPostgres(ctx, dsn)
	if err != nil {
		return err
	}
	defer pg.Close()
	admin := os.Getenv("PANEL_ADMIN_USER")
	if admin == "" {
		admin = "admin"
	}
	user := pg.UserByUsername(admin)
	if user == nil {
		return nil
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	user.MustChangePassword = false
	pg.PutUser(user)
	pg.RevokeSessionsForUser(user.ID)
	return nil
}
