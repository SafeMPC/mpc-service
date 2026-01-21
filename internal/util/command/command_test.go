package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SafeMPC/mpc-service/internal/api"
	"github.com/SafeMPC/mpc-service/internal/test"
	"github.com/SafeMPC/mpc-service/internal/util/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithServer(t *testing.T) {
	test.WithTestServer(t, func(s *api.Server) {
		ctx := t.Context()

		var testError = errors.New("test error")

		s.Config.Logger.PrettyPrintConsole = false
		resultErr := command.WithServer(ctx, s.Config, func(ctx context.Context, s *api.Server) error {
			var database string
			err := s.DB.QueryRowContext(ctx, "SELECT current_database();").Scan(&database)
			require.NoError(t, err)

			assert.NotEmpty(t, database)

			return testError
		})

		assert.Equal(t, testError, resultErr)
	})
}
