//go:build unit

package config_test

import (
	"testing"

	"github.com/hpcsc/svcview/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGroup(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		t.Run("a group has the name it was given, and holds each service whose name its match matches", func(t *testing.T) {
			workers, err := config.NewGroup("Workers", `^com\.example\.worker\.`)
			require.NoError(t, err)
			settings := config.Settings{Groups: []config.Group{workers}}

			require.Equal(t, map[string]string{
				"com.example.worker.01": "Workers",
				"com.example.api":       "Other",
			}, groupOfEach(settings, "com.example.worker.01", "com.example.api"))
		})

		t.Run("a match that is not a Go regular expression gives an error that names the group", func(t *testing.T) {
			_, err := config.NewGroup("Databases", "redis(")

			require.ErrorContains(t, err, `group "Databases"`)
		})

		t.Run("a group with no name, a group with the name Other, and a group with no match give an error", func(t *testing.T) {
			for name, match := range map[string]string{"": "worker", "Other": "worker", "Workers": ""} {
				_, err := config.NewGroup(name, match)

				require.Error(t, err, "name %q, match %q", name, match)
			}
		})
	})

	t.Run("match", func(t *testing.T) {
		t.Run("a group with no match, as the group Other, holds no service", func(t *testing.T) {
			settings := config.Settings{Groups: []config.Group{{Name: "Workers"}, config.Other}}

			require.Equal(t, map[string]string{
				"com.example.worker.01": "Other",
				"":                      "Other",
			}, groupOfEach(settings, "com.example.worker.01", ""))
		})
	})

	t.Run("short name", func(t *testing.T) {
		t.Run("is the part of the name after the text that the match matched, when the match starts at the first character, and else the full name", func(t *testing.T) {
			shortNames := map[[2]string]string{}
			for _, match := range []string{`^com\.example\.worker\.`, `com\.example\.`, `worker`, `^com\.example\.worker\.01$`, `.*`, `redis`, `x*`} {
				group, err := config.NewGroup("Workers", match)
				require.NoError(t, err)
				shortNames[[2]string{match, "com.example.worker.01"}] = group.ShortName("com.example.worker.01")
			}

			require.Equal(t, map[[2]string]string{
				{`^com\.example\.worker\.`, "com.example.worker.01"}:    "01",
				{`com\.example\.`, "com.example.worker.01"}:             "worker.01",
				{`worker`, "com.example.worker.01"}:                     "com.example.worker.01",
				{`^com\.example\.worker\.01$`, "com.example.worker.01"}: "com.example.worker.01",
				{`.*`, "com.example.worker.01"}:                         "com.example.worker.01",
				{`redis`, "com.example.worker.01"}:                      "com.example.worker.01",
				{`x*`, "com.example.worker.01"}:                         "com.example.worker.01",
			}, shortNames)
		})

		t.Run("in the group Other is the full name", func(t *testing.T) {
			require.Equal(t, "com.example.worker.01", config.Other.ShortName("com.example.worker.01"))
		})
	})
}
