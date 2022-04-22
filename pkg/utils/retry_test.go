/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License
 * as published by the Free Software Foundation; either version 2
 * of the License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, 0, true, 1000*time.Millisecond, func() (bool, error) {
		return false, nil
	})
	assert.Equal(t, ErrCancelled, err)
}

func TestRetryWithSuccess(t *testing.T) {
	err := Retry(context.Background(), 0, true, 10*time.Millisecond, func() (bool, error) {
		return true, nil
	})
	assert.NoError(t, err)
}

func TestRetryMaxAttempts(t *testing.T) {
	var list []string

	err := Retry(context.Background(), 3, true, 10*time.Millisecond, func() (bool, error) {
		list = append(list, "done")

		return false, nil
	})
	require.Error(t, err)
	assert.Equal(t, ErrReachMaxAttempts, err)
}

func TestRetryWithError(t *testing.T) {
	err := Retry(context.Background(), 3, true, 10*time.Millisecond, func() (bool, error) {
		return false, errors.New("bad")
	})
	require.Error(t, err)
	assert.Equal(t, "bad", err.Error())
}

func TestRetryWithTimeout(t *testing.T) {
	err := RetryWithTimeout(context.Background(), 50*time.Millisecond, 30*time.Millisecond, func() (bool, error) {
		return false, nil
	})
	require.Error(t, err)
	assert.Equal(t, ErrCancelled, err)
}
