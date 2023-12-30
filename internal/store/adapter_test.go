/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package store

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/test"
	"github.com/sapcc/go-bits/assert"
	"github.com/sapcc/go-bits/errext"
)

// NOTE: The database contents in these tests are all very minimal. The point
// of these tests is the FS handling, not encoding of specific objects.

var (
	//go:embed fixtures/db1.json
	db1Representation string
	db1Contents       = core.Database{
		Groups: []core.Group{{
			Name:             "nobody",
			LongName:         "Nobody in here.",
			MemberLoginNames: core.GroupMemberNames{},
		}},
		Users: []core.User{},
	}

	//go:embed fixtures/db2.json
	db2Representation string
	db2Contents       = core.Database{
		Groups: []core.Group{{
			Name:             "nobody",
			LongName:         "Still empty.",
			MemberLoginNames: core.GroupMemberNames{},
		}},
		Users: []core.User{},
	}

	//go:embed fixtures/db-autoinit.json
	autoinitDBRepresentation string
	autoinitDBContents       = core.Database{
		Groups: []core.Group{{
			Name:             "admins",
			LongName:         "Portunus Administrators",
			MemberLoginNames: core.GroupMemberNames{"admin": true},
			Permissions: core.Permissions{
				Portunus: core.PortunusPermissions{IsAdmin: true},
			},
		}},
		Users: []core.User{{
			LoginName:    "admin",
			GivenName:    "Initial",
			FamilyName:   "Administrator",
			PasswordHash: "<variable>",
		}},
	}
)

func TestReadExistingStore(t *testing.T) {
	vcfg := core.GetValidationConfigForTests()
	nexus := core.NewNexus(nil, vcfg, &core.NoopHasher{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dirPath, storePath := setupTempDir(t)
	defer os.RemoveAll(dirPath)

	//before starting, there are already contents in the database store
	test.ExpectNoError(t, os.WriteFile(storePath, []byte(db1Representation), 0666))

	//when the adapter loads those contents, verify that they decode into the expected DB contents
	nexus.AddListener(ctx, func(actualDB core.Database) {
		assert.DeepEqual(t, "database contents after load", actualDB, db1Contents)
		cancel() //make adapter.Run() return
	})

	//let the adapter load those contents
	adapter := NewAdapter(nexus, storePath)
	test.ExpectNoError(t, adapter.Run(ctx))
}

func TestReadSideloadedStore(t *testing.T) {
	vcfg := core.GetValidationConfigForTests()
	nexus := core.NewNexus(nil, vcfg, &core.NoopHasher{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dirPath, err := os.MkdirTemp(os.TempDir(), "portunus-storetest-")
	test.ExpectNoError(t, err)
	defer func() {
		test.ExpectNoError(t, os.RemoveAll(dirPath))
	}()
	storePath := filepath.Join(dirPath, "database.json")

	//before starting, there are already contents in the database store
	test.ExpectNoError(t, os.WriteFile(storePath, []byte(db1Representation), 0666))

	//when the adapter runs, we expect to see these contents on the first update,
	//then the sideloaded contents on the second update
	updateCount := 0
	nexus.AddListener(ctx, func(actualDB core.Database) {
		updateCount++
		switch updateCount {
		case 1:
			assert.DeepEqual(t, "database contents after initial load", actualDB, db1Contents)
		case 2:
			assert.DeepEqual(t, "database contents after sideload", actualDB, db2Contents)
			cancel() //make adapter.Run() return
		default:
			t.Error("too many updates")
		}
	})

	//while the adapter is running...
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		adapter := NewAdapter(nexus, storePath)
		test.ExpectNoError(t, adapter.Run(ctx))
	}()

	//...first we let it finish its startup...
	time.Sleep(25 * time.Millisecond)
	//...then we sideload different database contents into the store file
	test.ExpectNoError(t, os.WriteFile(storePath, []byte(db2Representation), 0666))

	//the listener above should cancel() after this change, so the adapter should shutdown
	wg.Wait()

	//verify that the sideloaded change was not overwritten by the adapter
	buf, err := os.ReadFile(storePath)
	test.ExpectNoError(t, err)
	assert.DeepEqual(t, "database contents after write", string(buf), db2Representation)
}

func TestWriteStore(t *testing.T) {
	vcfg := core.GetValidationConfigForTests()
	nexus := core.NewNexus(nil, vcfg, &core.NoopHasher{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dirPath, err := os.MkdirTemp(os.TempDir(), "portunus-storetest-")
	test.ExpectNoError(t, err)
	defer func() {
		test.ExpectNoError(t, os.RemoveAll(dirPath))
	}()
	storePath := filepath.Join(dirPath, "database.json")

	//before starting, there are already contents in the database store
	test.ExpectNoError(t, os.WriteFile(storePath, []byte(db1Representation), 0666))

	//we don't care about these initial contents, but we need the adapter running
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		adapter := NewAdapter(nexus, storePath)
		test.ExpectNoError(t, adapter.Run(ctx))
	}()

	//after the adapter has set up its listener...
	time.Sleep(25 * time.Millisecond)
	//...we update the database
	action := func(db *core.Database) errext.ErrorSet {
		*db = db2Contents.Cloned()
		return nil
	}
	errs := nexus.Update(action, nil)
	for _, err := range errs {
		test.ExpectNoError(t, err)
	}

	//after the adapter has reacted to this update...
	time.Sleep(25 * time.Millisecond)
	//...shut it down and check that it wrote the right data into the store
	cancel()
	wg.Wait()
	buf, err := os.ReadFile(storePath)
	test.ExpectNoError(t, err)
	assert.DeepEqual(t, "database contents after write", string(buf), db2Representation)
}

func TestInitializeMissingStore(t *testing.T) {
	vcfg := core.GetValidationConfigForTests()
	nexus := core.NewNexus(nil, vcfg, &core.NoopHasher{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dirPath, storePath := setupTempDir(t)
	defer os.RemoveAll(dirPath)

	//when the adapter starts up and finds no store...
	var wg1 sync.WaitGroup
	wg1.Add(1)
	var realPasswordHash string
	nexus.AddListener(ctx, func(actualDB core.Database) {
		//...the nexus will auto-initialize a DB with an initial admin account
		for idx := range actualDB.Users {
			realPasswordHash = actualDB.Users[idx].PasswordHash
			actualDB.Users[idx].PasswordHash = "<variable>"
		}
		assert.DeepEqual(t, "database contents after load", actualDB, autoinitDBContents)
		wg1.Done()
	})

	//let the adapter fulfil this promise
	adapter := NewAdapter(nexus, storePath)
	var wg2 sync.WaitGroup
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		test.ExpectNoError(t, adapter.Run(ctx))
	}()

	//wait for the load to be observed...
	wg1.Wait()
	//...then let the adapter finish its write and shut it down afterwards
	time.Sleep(25 * time.Millisecond)
	cancel()
	wg2.Wait()

	//check that the newly initialized DB was written correctly
	buf, err := os.ReadFile(storePath)
	test.ExpectNoError(t, err)
	repr := strings.Replace(string(buf), realPasswordHash, "<variable>", 1)
	assert.DeepEqual(t, "database contents after write", repr, autoinitDBRepresentation)
}

func setupTempDir(t *testing.T) (dirPath, storePath string) {
	dirPath, err := os.MkdirTemp(os.TempDir(), "portunus-storetest-")
	test.ExpectNoError(t, err)
	return dirPath, filepath.Join(dirPath, "database.json")
}
