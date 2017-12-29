// Copyright (c) 2017, Xiaomi, Inc.  All rights reserved.
// This source code is licensed under the Apache License Version 2.0, which
// can be found in the LICENSE file in the root directory of this source tree.

package pegasus

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
)

func TestPegasusClient_OpenTable(t *testing.T) {
	defer leaktest.Check(t)()

	cfg := Config{
		MetaServers: []string{"0.0.0.0:34601", "0.0.0.0:34602", "0.0.0.0:34603"},
	}

	client := NewClient(cfg)
	defer client.Close()

	tb1, err := client.OpenTable(context.Background(), "temp")
	assert.Nil(t, err)
	assert.NotNil(t, tb1)

	tb2, err := client.OpenTable(context.Background(), "temp")
	assert.Nil(t, err)
	assert.NotNil(t, tb1)

	// must reuse previous connection
	assert.Equal(t, tb1, tb2)

	pclient, _ := client.(*pegasusClient)
	assert.NotNil(t, pclient.findTable("temp"))

	tb, err := client.OpenTable(context.Background(), "table_not_exists")
	assert.NotNil(t, err)
	assert.Nil(t, tb)
}

func TestPegasusClient_OpenTableTimeout(t *testing.T) {
	defer leaktest.Check(t)()

	// make sure the port 8801 is not opened on your computer.
	cfg := Config{
		MetaServers: []string{"0.0.0.0:8801"},
	}

	client := NewClient(cfg)

	ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
	tb, err := client.OpenTable(ctx, "temp")
	assert.Equal(t, ctx.Err(), context.DeadlineExceeded)
	assert.Nil(t, tb)
	assert.NotNil(t, err)

	client.Close()
}

// Ensure that concurrent OpenTable operations to the same table
// won't invoke more than one query to meta server.
func TestPegasusClient_ConcurrentOpenSameTable(t *testing.T) {
	defer leaktest.Check(t)()

	cfg := Config{
		MetaServers: []string{"0.0.0.0:34601", "0.0.0.0:34602", "0.0.0.0:34603"},
	}
	client := NewClient(cfg)
	defer client.Close()

	var tblist []TableConnector
	openTableQueries := 100

	var wg sync.WaitGroup
	for i := 0; i < openTableQueries; i++ {
		wg.Add(1)
		go func() {
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			tb, err := client.OpenTable(ctx, "temp")
			assert.Nil(t, err)
			tblist = append(tblist, tb)
			wg.Done()
		}()
	}
	wg.Wait()

	// all tables returned by OpenTable must be the same one
	tb := tblist[0]
	for i := 1; i < openTableQueries; i++ {
		assert.Equal(t, tb, tblist[i])
	}
}

// In this test we verifies if there's any easy bugs can be found in concurrent rpc.
func TestPegasusClient_ConcurrentMetaQueries(t *testing.T) {
	defer leaktest.Check(t)()

	cfg := Config{
		MetaServers: []string{"0.0.0.0:34601", "0.0.0.0:34602", "0.0.0.0:34603"},
	}
	client := NewClient(cfg)
	defer client.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)

		id := i
		go func() {
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			_, err := client.OpenTable(ctx, "table_not_exist"+fmt.Sprint(id))
			assert.NotNil(t, err)
			wg.Done()
		}()
	}
	wg.Wait()
}

// Produce larger workload and test if anything goes wrong.
func TestPegasusClient_SequentialOperations(t *testing.T) {
	defer leaktest.Check(t)()

	cfg := Config{
		MetaServers: []string{"0.0.0.0:34601", "0.0.0.0:34602", "0.0.0.0:34603"},
	}

	client := NewClient(cfg)
	defer client.Close()

	for i := 0; i < 1000; i++ {
		ctx, _ := context.WithTimeout(context.Background(), time.Second*3)
		hashKey := []byte(fmt.Sprintf("h%d", i))
		sortKey := []byte(fmt.Sprintf("s%d", i))
		value := []byte(fmt.Sprintf("v%d", i))

		err := client.Set(ctx, "temp", hashKey, sortKey, value)
		assert.Nil(t, err)

		actual, err := client.Get(ctx, "temp", hashKey, sortKey)
		assert.Nil(t, err)
		assert.Equal(t, actual, value)

		err = client.Del(ctx, "temp", hashKey, sortKey)
		assert.Nil(t, err)
	}
}

func TestPegasusClient_ConcurrentSet(t *testing.T) {
	defer leaktest.Check(t)()

	cfg := Config{
		MetaServers: []string{"0.0.0.0:34601", "0.0.0.0:34602", "0.0.0.0:34603"},
	}

	client := NewClient(cfg)
	defer client.Close()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)

		id := i
		go func() {
			ctx, _ := context.WithTimeout(context.Background(), time.Second*3)
			hashKey := []byte(fmt.Sprintf("h%d", id))
			sortKey := []byte(fmt.Sprintf("s%d", id))
			value := []byte(fmt.Sprintf("v%d", id))

			err := client.Set(ctx, "temp", hashKey, sortKey, value)
			assert.Nil(t, err)

			wg.Done()
		}()
	}
	wg.Wait()
}
