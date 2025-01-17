package meta_test

import (
	"github.com/cnosdb/cnosdb"
	"github.com/cnosdb/cnosdb/meta"
	"github.com/cnosdb/cnosdb/vend/cnosql"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMetaClient_CreateDatabaseOnly(t *testing.T) {
	t.Parallel()

	dir, c := newClient()

	defer os.RemoveAll(dir)
	defer c.Close()

	if db, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	} else if db.Name != "db0" {
		t.Fatalf("database name mismatch.  exp: db0, got %s", db.Name)
	}

	db := c.Database("db0")
	if db == nil {
		t.Fatal("database is not existing")
	} else if db.Name != "db0" {
		t.Fatalf("db name is wrong.")
	}

	rp, err := c.RetentionPolicy("db0", "autogen")
	if err != nil {
		t.Fatal(err)
	} else if rp == nil {
		t.Fatalf("retention policy is not existing.")
	} else if rp.Name != "autogen" {
		t.Fatalf("retention policy mismatch. exp:autogen, got %s", rp.Name)
	}
}

func TestMetaClient_CreateDatabaseIfNotExists(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	db := c.Database("db0")
	if db == nil {
		t.Fatal("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	rp, err := c.RetentionPolicy("db0", "autogen")
	if err != nil {
		t.Fatal(err)
	} else if rp == nil {
		t.Fatalf("retention policy is not existing.")
	} else if rp.Name != "autogen" {
		t.Fatalf("retention policy mismatch. exp:autogen, got %s", rp.Name)
	}
}

func TestMetaClient_CreateDatabaseWithRetentionPolicy(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", nil); err == nil {
		t.Fatal("expected error")
	}

	duration := 1 * time.Hour
	replicaN := 1
	spec := meta.RetentionPolicySpec{
		Name:               "rp0",
		Duration:           &duration,
		ReplicaN:           &replicaN,
		ShardGroupDuration: 60 * time.Minute,
	}
	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec); err != nil {
		t.Fatal(err)
	}

	db := c.Database("db0")
	if db == nil {
		t.Fatal("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	rp := db.RetentionPolicy("rp0")
	if rp.Name != "rp0" {
		t.Fatalf("rp name wrong: %s", rp.Name)
	} else if rp.Duration != time.Hour {
		t.Fatalf("rp duration wrong: %v", rp.Duration)
	} else if rp.ReplicaN != 1 {
		t.Fatalf("rp replication wrong: %d", rp.ReplicaN)
	} else if rp.ShardGroupDuration != 60*time.Minute {
		t.Fatalf("rp shard duration wrong: %v", rp.ShardGroupDuration)
	}

	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec); err != nil {
		t.Fatal(err)
	}

	if db0, err := c.CreateDatabase("db0"); err != nil {
		t.Fatalf("got %v, but exp %v", err, nil)
	} else if db0.DefaultRetentionPolicy != "rp0" {
		t.Fatalf("got %v, but exp %v", db0.DefaultRetentionPolicy, "rp0")
	} else if got, exp := len(db0.RetentionPolicies), 1; got != exp {
		t.Fatalf("got %v, but exp %v", got, exp)
	}
}

func TestMetaClient_CreateDatabaseWithRetentionPolicy_Conflict_Fields(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	duration := 1 * time.Hour
	replicaN := 1
	spec := meta.RetentionPolicySpec{
		Name:               "rp0",
		Duration:           &duration,
		ReplicaN:           &replicaN,
		ShardGroupDuration: 60 * time.Minute,
	}
	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec); err != nil {
		t.Fatal(err)
	}

	spec2 := spec
	spec2.Name = spec.Name + "1"
	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec2); err != meta.ErrRetentionPolicyConflict {
		t.Fatalf("got %v, but expected %v", err, meta.ErrRetentionPolicyConflict)
	}

	spec2 = spec
	duration2 := *spec.Duration + time.Minute
	spec2.Duration = &duration2
	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec2); err != meta.ErrRetentionPolicyConflict {
		t.Fatalf("got %v, but expected %v", err, meta.ErrRetentionPolicyConflict)
	}

	spec2 = spec
	replica2 := *spec.ReplicaN + 1
	spec2.ReplicaN = &replica2
	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec2); err != meta.ErrRetentionPolicyConflict {
		t.Fatalf("got %v, but expected %v", err, meta.ErrRetentionPolicyConflict)
	}

	spec2 = spec
	spec2.ShardGroupDuration = spec.ShardGroupDuration + time.Minute
	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec2); err != meta.ErrRetentionPolicyConflict {
		t.Fatalf("got %v, but expected %v", err, meta.ErrRetentionPolicyConflict)
	}
}

func TestMetaClient_CreateDatabaseWithRetentionPolicy_Conflict_NonDefault(t *testing.T) {
	t.Parallel()

	d, c := newClient()
	defer os.RemoveAll(d)
	defer c.Close()

	duration := 1 * time.Hour
	replicaN := 1
	spec := meta.RetentionPolicySpec{
		Name:               "rp0",
		Duration:           &duration,
		ReplicaN:           &replicaN,
		ShardGroupDuration: 60 * time.Minute,
	}

	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec); err != nil {
		t.Fatal(err)
	}

	spec2 := spec
	spec2.Name = "rp1"
	if _, err := c.CreateRetentionPolicy("db0", &spec2, false); err != nil {
		t.Fatal(err)
	}

	//notice that makeDefault field is not set,that's error.
	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &spec2); err != meta.ErrRetentionPolicyConflict {
		t.Fatalf("got %v, but expected %v", err, meta.ErrRetentionPolicyConflict)
	}
}

func TestMetaClient_Databases(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	// Create three databases.
	db, err := c.CreateDatabase("db0")
	if err != nil {
		t.Fatal(err)
	} else if db == nil {
		t.Fatal("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	db, err = c.CreateDatabase("db1")
	if err != nil {
		t.Fatal(err)
	} else if db.Name != "db1" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	db, err = c.CreateDatabase("db2")
	if err != nil {
		t.Fatal(err)
	} else if db.Name != "db2" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	dbs := c.Databases()
	if err != nil {
		t.Fatal(err)
	}
	if len(dbs) != 3 {
		t.Fatalf("expected 2 databases but got %d", len(dbs))
	} else if dbs[0].Name != "db0" {
		t.Fatalf("db name wrong: %s", dbs[0].Name)
	} else if dbs[1].Name != "db1" {
		t.Fatalf("db name wrong: %s", dbs[1].Name)
	} else if dbs[2].Name != "db2" {
		t.Fatalf("db name wrong: %s", dbs[2].Name)
	}
}

func TestMetaClient_DropDatabase(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	db := c.Database("db0")
	if db == nil {
		t.Fatalf("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	if err := c.DropDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	if db = c.Database("db0"); db != nil {
		t.Fatalf("expected database to no return: %v", db)
	}

	if err := c.DropDatabase("db foo"); err != nil {
		t.Fatalf("got %v error, but expected no error", err)
	}
}

func TestMetaClient_CreateRetentionPolicy(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	db := c.Database("db0")
	if db == nil {
		t.Fatal("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	rp0 := meta.RetentionPolicyInfo{
		Name:               "rp0",
		ReplicaN:           1,
		Duration:           2 * time.Hour,
		ShardGroupDuration: 2 * time.Hour,
	}

	rp0Spec := &meta.RetentionPolicySpec{
		Name:               rp0.Name,
		ReplicaN:           &rp0.ReplicaN,
		Duration:           &rp0.Duration,
		ShardGroupDuration: rp0.ShardGroupDuration,
	}

	if _, err := c.CreateRetentionPolicy("db0", rp0Spec, true); err != nil {
		t.Fatal(err)
	}

	actual, err := c.RetentionPolicy("db0", "rp0")
	if err != nil {
		t.Fatal(err)
	} else if got, exp := actual, &rp0; !reflect.DeepEqual(got, exp) {
		t.Fatalf("got %#v, expected %#v", got, exp)
	}

	if _, err := c.CreateRetentionPolicy("db0", rp0Spec, true); err != nil {
		t.Fatal(err)
	} else if actual, err = c.RetentionPolicy("db0", "rp0"); err != nil {
		t.Fatal(err)
	} else if got, exp := actual, &rp0; !reflect.DeepEqual(got, exp) {
		t.Fatalf("got %#v, expected %#v", got, exp)
	}

	rp1 := rp0
	rp1.Duration = 2 * rp0.Duration

	rp1Spec := &meta.RetentionPolicySpec{
		Name:               rp1.Name,
		ReplicaN:           &rp1.ReplicaN,
		Duration:           &rp1.Duration,
		ShardGroupDuration: rp1.ShardGroupDuration,
	}
	_, got := c.CreateRetentionPolicy("db0", rp1Spec, true)
	if exp := meta.ErrRetentionPolicyExists; got != exp {
		t.Fatalf("got error %v, expected error %v", got, exp)
	}

	rp1Spec = rp0Spec
	*rp1Spec.ReplicaN = *rp0Spec.ReplicaN + 1

	_, got = c.CreateRetentionPolicy("db0", rp1Spec, true)
	if exp := meta.ErrRetentionPolicyExists; got != exp {
		t.Fatalf("got error %v, expected error %v", got, exp)
	}

	rp1Spec = rp0Spec
	rp1Spec.ShardGroupDuration = rp0Spec.ShardGroupDuration / 2

	_, got = c.CreateRetentionPolicy("db0", rp1Spec, true)
	if exp := meta.ErrRetentionPolicyExists; got != exp {
		t.Fatalf("got error %v, expected error %v", got, exp)
	}

	rp1Spec = rp0Spec
	*rp1Spec.Duration = 1 * time.Hour
	rp1Spec.ShardGroupDuration = 2 * time.Hour

	_, got = c.CreateRetentionPolicy("db0", rp1Spec, true)
	if exp := meta.ErrIncompatibleDurations; got != exp {
		t.Fatalf("got error %v, expected error %v", got, exp)
	}
}

func TestMetaClient_DefaultRetentionPolicy(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	duration := 1 * time.Hour
	replicaN := 1
	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &meta.RetentionPolicySpec{
		Name:     "rp0",
		Duration: &duration,
		ReplicaN: &replicaN,
	}); err != nil {
		t.Fatal(err)
	}

	db := c.Database("db0")
	if db == nil {
		t.Fatal("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	rp, err := c.RetentionPolicy("db0", "rp0")
	if err != nil {
		t.Fatal(err)
	} else if rp.Name != "rp0" {
		t.Fatalf("rp name wrong: %s", rp.Name)
	} else if rp.Duration != time.Hour {
		t.Fatalf("rp duration wrong: %s", rp.Duration.String())
	} else if rp.ReplicaN != 1 {
		t.Fatalf("rp replication wrong: %d", rp.ReplicaN)
	}

	if exp, got := "rp0", db.DefaultRetentionPolicy; exp != got {
		t.Fatalf("rp name wrong: \n\texp: %s\n\tgot: %s", exp, db.DefaultRetentionPolicy)
	}
}

func TestMetaClient_SetDefaultRetentionPolicy(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	// Create a database.
	db, err := c.CreateDatabase("db0")
	if err != nil {
		t.Fatal(err)
	} else if db == nil {
		t.Fatal("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	rp0 := meta.RetentionPolicyInfo{
		Name:               "rp0",
		ReplicaN:           1,
		Duration:           2 * time.Hour,
		ShardGroupDuration: 2 * time.Hour,
	}

	if _, err := c.CreateRetentionPolicy("db0", &meta.RetentionPolicySpec{
		Name:               rp0.Name,
		ReplicaN:           &rp0.ReplicaN,
		Duration:           &rp0.Duration,
		ShardGroupDuration: rp0.ShardGroupDuration,
	}, true); err != nil {
		t.Fatal(err)
	}

	err = c.SetDefaultRetentionPolicy("db0", "rp0")
	if err != nil {
		t.Fatal(err)
	}

	db0 := c.Database("db0")
	if db0.DefaultRetentionPolicy != "rp0" {
		t.Fatalf("database default rp is wrong. exp: rp0 got: %s", db0.DefaultRetentionPolicy)
	}

}

func TestMetaClient_UpdateRetentionPolicy(t *testing.T) {
	t.Parallel()

	d, c := newClient()
	defer os.RemoveAll(d)
	defer c.Close()

	if _, err := c.CreateDatabaseWithRetentionPolicy("db0", &meta.RetentionPolicySpec{
		Name:               "rp0",
		ShardGroupDuration: 4 * time.Hour,
	}); err != nil {
		t.Fatal(err)
	}

	rpi, err := c.RetentionPolicy("db0", "rp0")
	if err != nil {
		t.Fatal(err)
	}

	duration := 2 * rpi.ShardGroupDuration
	replicaN := 1
	if err := c.UpdateRetentionPolicy("db0", "rp0", &meta.RetentionPolicyUpdate{
		Duration: &duration,
		ReplicaN: &replicaN,
	}, true); err != nil {
		t.Fatal(err)
	}

	rpi, err = c.RetentionPolicy("db0", "rp0")
	if err != nil {
		t.Fatal(err)
	}
	if exp, got := 4*time.Hour, rpi.ShardGroupDuration; exp != got {
		t.Fatalf("shard group duration wrong: \n\texp: %s\n\tgot: %s", exp, got)
	}

	duration = rpi.ShardGroupDuration / 2
	if err := c.UpdateRetentionPolicy("db0", "rp0", &meta.RetentionPolicyUpdate{
		Duration: &duration,
	}, true); err == nil {
		t.Fatal("expected error")
	} else if err != meta.ErrIncompatibleDurations {
		t.Fatalf("expected error '%s', got '%s'", meta.ErrIncompatibleDurations, err)
	}

	sgDuration := rpi.Duration * 2
	if err := c.UpdateRetentionPolicy("db0", "rp0", &meta.RetentionPolicyUpdate{
		ShardGroupDuration: &sgDuration,
	}, true); err == nil {
		t.Fatal("expected error")
	} else if err != meta.ErrIncompatibleDurations {
		t.Fatalf("expected error '%s', got '%s'", meta.ErrIncompatibleDurations, err)
	}

	duration = rpi.ShardGroupDuration
	sgDuration = rpi.Duration
	if err := c.UpdateRetentionPolicy("db0", "rp0", &meta.RetentionPolicyUpdate{
		Duration:           &duration,
		ShardGroupDuration: &sgDuration,
	}, true); err == nil {
		t.Fatal("expected error")
	} else if err != meta.ErrIncompatibleDurations {
		t.Fatalf("expected error '%s', got '%s'", meta.ErrIncompatibleDurations, err)
	}

	duration = time.Duration(0)
	sgDuration = 168 * time.Hour
	if err := c.UpdateRetentionPolicy("db0", "rp0", &meta.RetentionPolicyUpdate{
		Duration:           &duration,
		ShardGroupDuration: &sgDuration,
	}, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMetaClient_DropRetentionPolicy(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	db := c.Database("db0")
	if db == nil {
		t.Fatal("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	duration := 1 * time.Hour
	replicaN := 1
	if _, err := c.CreateRetentionPolicy("db0", &meta.RetentionPolicySpec{
		Name:     "rp0",
		Duration: &duration,
		ReplicaN: &replicaN,
	}, true); err != nil {
		t.Fatal(err)
	}

	rp, err := c.RetentionPolicy("db0", "rp0")
	if err != nil {
		t.Fatal(err)
	} else if rp.Name != "rp0" {
		t.Fatalf("rp name wrong: %s", rp.Name)
	} else if rp.Duration != time.Hour {
		t.Fatalf("rp duration wrong: %s", rp.Duration.String())
	} else if rp.ReplicaN != 1 {
		t.Fatalf("rp replication wrong: %d", rp.ReplicaN)
	}

	if err := c.DropRetentionPolicy("db0", "rp0"); err != nil {
		t.Fatal(err)
	}

	rp, err = c.RetentionPolicy("db0", "rp0")
	if err != nil {
		t.Fatal(err)
	} else if rp != nil {
		t.Fatalf("rp should have been dropped")
	}
}

func TestMetaClient_CreateUser(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateUser("Jerry", "supersecure", true); err != nil {
		t.Fatal(err)
	}

	if _, err := c.CreateUser("Tom", "password", false); err != nil {
		t.Fatal(err)
	}

	users := c.Users()
	if len(users) != 2 {
		t.Fatalf("The length of users should be 2")
	}

	exists := c.AdminUserExists()
	if !exists {
		t.Fatalf("admin should exist")
	}

	u, err := c.User("Jerry")
	if err != nil {
		t.Fatal(err)
	}
	if exp, got := "Jerry", u.ID(); exp != got {
		t.Fatalf("unexpected user name: exp: %s got: %s", exp, got)
	}
	if !isAdmin(u) {
		t.Fatalf("expected user to be admin")
	}

	u, err = c.Authenticate("Jerry", "supersecure")
	if u == nil || err != nil || u.ID() != "Jerry" {
		t.Fatalf("failed to authenticate")
	}

	u, err = c.Authenticate("Jerry", "badpassword")
	if u != nil || err != meta.ErrAuthenticate {
		t.Fatalf("authentication should fail with %s", meta.ErrAuthenticate)
	}

	u, err = c.Authenticate("Jerry", "")
	if u != nil || err != meta.ErrAuthenticate {
		t.Fatalf("authentication should fail with %s", meta.ErrAuthenticate)
	}

	if err := c.UpdateUser("Jerry", "moresupersecure"); err != nil {
		t.Fatal(err)
	}

	u, err = c.Authenticate("Jerry", "supersecure")
	if u != nil || err != meta.ErrAuthenticate {
		t.Fatalf("authentication should fail with %s", meta.ErrAuthenticate)
	}

	u, err = c.Authenticate("Jerry", "moresupersecure")
	if u == nil || err != nil || u.ID() != "Jerry" {
		t.Fatalf("failed to authenticate")
	}

	u, err = c.Authenticate("foo", "")
	if u != nil || err != meta.ErrUserNotFound {
		t.Fatalf("authentication should fail with %s", meta.ErrUserNotFound)
	}

	u, err = c.User("Tom")
	if err != nil {
		t.Fatal(err)
	}
	if exp, got := "Tom", u.ID(); exp != got {
		t.Fatalf("unexpected user name: exp: %s got: %s", exp, got)
	}
	if isAdmin(u) {
		t.Fatalf("expected user not to be an admin")
	}

	if exp, got := 2, c.UserCount(); exp != got {
		t.Fatalf("unexpected user count.  got: %d exp: %d", got, exp)
	}

	if err := c.SetAdminPrivilege("Tom", true); err != nil {
		t.Fatal(err)
	}

	u, err = c.User("Tom")
	if err != nil {
		t.Fatal(err)
	}
	if exp, got := "Tom", u.ID(); exp != got {
		t.Fatalf("unexpected user name: exp: %s got: %s", exp, got)
	}
	if !isAdmin(u) {
		t.Fatalf("expected user to be an admin")
	}

	if err := c.SetAdminPrivilege("Tom", false); err != nil {
		t.Fatal(err)
	}

	u, err = c.User("Tom")
	if err != nil {
		t.Fatal(err)
	}
	if exp, got := "Tom", u.ID(); exp != got {
		t.Fatalf("unexpected user name: exp: %s got: %s", exp, got)
	}
	if isAdmin(u) {
		t.Fatalf("expected user not to be an admin")
	}

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	db := c.Database("db0")
	if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	if err := c.SetPrivilege("Tom", "db0", cnosql.ReadPrivilege); err != nil {
		t.Fatal(err)
	}

	p, err := c.UserPrivilege("Tom", "db0")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected privilege but was nil")
	}
	if exp, got := cnosql.ReadPrivilege, *p; exp != got {
		t.Fatalf("unexpected privilege.  exp: %d, got: %d", exp, got)
	}

	if err := c.SetPrivilege("Tom", "db0", cnosql.NoPrivileges); err != nil {
		t.Fatal(err)
	}
	p, err = c.UserPrivilege("Tom", "db0")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected privilege but was nil")
	}
	if exp, got := cnosql.NoPrivileges, *p; exp != got {
		t.Fatalf("unexpected privilege.  exp: %d, got: %d", exp, got)
	}

	if err := c.DropUser("Tom"); err != nil {
		t.Fatal(err)
	}

	if _, err = c.User("Tom"); err != meta.ErrUserNotFound {
		t.Fatalf("user lookup should fail with %s", meta.ErrUserNotFound)
	}

	if exp, got := 1, c.UserCount(); exp != got {
		t.Fatalf("unexpected user count.  got: %d exp: %d", got, exp)
	}
}

func TestMetaClient_UserPrivileges(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateUser("Jerry", "supersecure", true); err != nil {
		t.Fatal(err)
	}

	if _, err := c.CreateUser("Tom", "password", false); err != nil {
		t.Fatal(err)
	}

	db, err := c.CreateDatabase("db0")
	if err != nil {
		return
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	if err := c.SetPrivilege("Tom", "db0", cnosql.ReadPrivilege); err != nil {
		t.Fatal(err)
	}

	if err := c.SetPrivilege("Jerry", "db0", cnosql.AllPrivileges); err != nil {
		t.Fatal(err)
	}

	privileges, err := c.UserPrivileges("Jerry")
	if err != nil {
		t.Fatal(err)
	} else if privileges == nil && privileges["db0"] != cnosql.AllPrivileges {
		t.Fatalf("User privileges mismatch.")
	}

}

func TestMetaClient_UpdateUser_Exists(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateUser("Jerry", "supersecure", true); err != nil {
		t.Fatal(err)
	}

	if err := c.UpdateUser("Jerry", "password"); err != nil {
		t.Fatal(err)
	}
}

func TestMetaClient_UpdateUser_NonExists(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if err := c.UpdateUser("foo", "bar"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestMetaClient_ContinuousQueries(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	// Create a database to use
	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}
	db := c.Database("db0")
	if db == nil {
		t.Fatalf("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	if err := c.CreateContinuousQuery("db0", "cq0", `SELECT count(value) INTO foo_count FROM foo GROUP BY time(10m)`); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateContinuousQuery("db0", "cq0", `SELECT count(value) INTO foo_count FROM foo GROUP BY time(10m)`); err != nil {
		t.Fatalf("got error %q, but didn't expect one", err)
	}

	if err := c.CreateContinuousQuery("db0", "cq0", `SELECT min(value) INTO foo_max FROM foo GROUP BY time(20m)`); err == nil {
		t.Fatal("didn't get and error, but expected one")
	} else if got, exp := err, meta.ErrContinuousQueryExists; got.Error() != exp.Error() {
		t.Fatalf("got %v, expected %v", got, exp)
	}
	if err := c.CreateContinuousQuery("db0", "cq1", `SELECT max(value) INTO foo_max FROM foo GROUP BY time(10m)`); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateContinuousQuery("db0", "cq2", `SELECT min(value) INTO foo_min FROM foo GROUP BY time(10m)`); err != nil {
		t.Fatal(err)
	}
	//This is meta, we don't need to execute the CQ query
	if err := c.DropContinuousQuery("db0", "cq1"); err != nil {
		t.Fatal(err)
	}

	if err := c.DropContinuousQuery("db0", "not-a-cq"); err != nil {
		t.Fatal(err)
	}
}

func TestMetaClient_Subscriptions_Create(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	// Create a database to use
	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}
	db := c.Database("db0")
	if db == nil {
		t.Fatal("database not found")
	} else if db.Name != "db0" {
		t.Fatalf("db name wrong: %s", db.Name)
	}

	if err := c.CreateSubscription("db0", "autogen", "sub0", "ALL", []string{"udp://example.com:9090"}); err != nil {
		t.Fatal(err)
	}

	err := c.CreateSubscription("db0", "autogen", "sub0", "ALL", []string{"udp://example.com:9090"})
	if err == nil || err.Error() != `subscription already exists` {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := c.CreateSubscription("db0", "autogen", "sub1", "ALL", []string{"udp://example.com:6060"}); err != nil {
		t.Fatal(err)
	}

	err = c.CreateSubscription("db0", "autogen", "sub2", "ALL", []string{"bad://example.com:9191"})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid subscription URL") {
		t.Fatalf("unexpected error: %s", err)
	}

	err = c.CreateSubscription("db0", "autogen", "sub2", "ALL", []string{"udp://example.com"})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid subscription URL") {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := c.CreateSubscription("db0", "autogen", "sub3", "ALL", []string{"http://example.com:9092"}); err != nil {
		t.Fatal(err)
	}

	if err := c.CreateSubscription("db0", "autogen", "sub4", "ALL", []string{"https://example.com:9092"}); err != nil {
		t.Fatal(err)
	}
}

func TestMetaClient_Subscriptions_Drop(t *testing.T) {
	t.Parallel()

	d, c := newClient()
	defer os.RemoveAll(d)
	defer c.Close()

	// Create a database to use
	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	err := c.DropSubscription("db0", "autogen", "foo")
	if got, exp := err, meta.ErrSubscriptionNotFound; got == nil || got.Error() != exp.Error() {
		t.Fatalf("got: %s, exp: %s", got, exp)
	}

	if err := c.CreateSubscription("db0", "autogen", "sub0", "ALL", []string{"udp://example.com:9090"}); err != nil {
		t.Fatal(err)
	}

	err = c.DropSubscription("foo", "autogen", "sub0")
	if got, exp := err, cnosdb.ErrDatabaseNotFound("foo"); got.Error() != exp.Error() {
		t.Fatalf("got: %s, exp: %s", got, exp)
	}

	err = c.DropSubscription("db0", "foo_policy", "sub0")
	if got, exp := err, cnosdb.ErrRetentionPolicyNotFound("foo_policy"); got.Error() != exp.Error() {
		t.Fatalf("got: %s, exp: %s", got, exp)
	}

	err = c.DropSubscription("db0", "autogen", "sub0")
	if got := err; got != nil {
		t.Fatalf("got: %s, exp: %v", got, nil)
	}
}

func TestMetaClient_Shards(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	// Test creating a shard group.
	tmin := time.Now()
	sg, err := c.CreateShardGroup("db0", "autogen", tmin)
	if err != nil {
		t.Fatal(err)
	} else if sg == nil {
		t.Fatalf("expected ShardGroup")
	}

	// Test pre-creating shard groups.
	dur := sg.EndTime.Sub(sg.StartTime) + time.Nanosecond
	tmax := tmin.Add(dur)
	if err := c.PrecreateShardGroups(tmin, tmax); err != nil {
		t.Fatal(err)
	}

	// Test finding shard groups by time range.
	groups, err := c.ShardGroupsByTimeRange("db0", "autogen", tmin, tmax)
	if err != nil {
		t.Fatal(err)
	} else if len(groups) != 2 {
		t.Fatalf("wrong number of shard groups: %d", len(groups))
	}

	// Test finding shard owner.
	db, rp, owner := c.ShardOwner(groups[0].Shards[0].ID)
	if db != "db0" {
		t.Fatalf("wrong db name: %s", db)
	} else if rp != "autogen" {
		t.Fatalf("wrong rp name: %s", rp)
	} else if owner.ID != groups[0].ID {
		t.Fatalf("wrong owner: exp %d got %d", groups[0].ID, owner.ID)
	}

	addOwners := []uint64{22, 33}
	var delOwners []uint64
	err = c.UpdateShardOwners(groups[0].Shards[0].ID, addOwners, delOwners)
	if err != nil {
		return
	}
	groups, err = c.ShardGroupsByTimeRange("db0", "autogen", tmin, tmax)
	owners := groups[0].Shards[0].Owners

	if owners[0].NodeID != 0 && owners[1].NodeID != 22 && owners[2].NodeID != 33 {
		t.Fatalf("UpdateShardOwners failed")
	}

	delOwners = append(delOwners, 22)
	err = c.UpdateShardOwners(groups[0].Shards[0].ID, addOwners, delOwners)
	if err != nil {
		return
	}
	groups, err = c.ShardGroupsByTimeRange("db0", "autogen", tmin, tmax)
	owners = groups[0].Shards[0].Owners

	if owners[0].NodeID != 0 && owners[1].NodeID != 33 {
		t.Fatalf("UpdateShardOwners failed")
	}

	// Test deleting a shard group.
	if err := c.DeleteShardGroup("db0", "autogen", groups[0].ID); err != nil {
		t.Fatal(err)
	} else if groups, err = c.ShardGroupsByTimeRange("db0", "autogen", tmin, tmax); err != nil {
		t.Fatal(err)
	} else if len(groups) != 1 {
		t.Fatalf("wrong number of shard groups after delete: %d", len(groups))
	}
}

func TestMetaClient_DropShard(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	// Test creating a shard group.
	tmin := time.Now()
	sg, err := c.CreateShardGroup("db0", "autogen", tmin)
	if err != nil {
		t.Fatal(err)
	} else if sg == nil {
		t.Fatalf("expected ShardGroup")
	}

	//create a shard group with a shard
	dur := sg.EndTime.Sub(sg.StartTime)
	tmax := tmin.Add(dur)
	sds, err := c.ShardGroupsByTimeRange("db0", "autogen", tmin, tmax)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(sds)

	//delete all shards
	for i := 0; i < len(sds); i++ {
		for j := 0; j < len(sds[i].Shards); j++ {
			err := c.DropShard(sds[i].Shards[j].ID)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	sds, err = c.ShardGroupsByTimeRange("db0", "autogen", tmin, tmax)
	if err != nil {
		t.Fatal(err)
	} else if len(sds) != 0 {
		t.Fatalf("shard group should be empty")
	}
	t.Log(sds)

}

func TestMetaClient_PrecreateShardGroups(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	// creat a shard group.
	tmin := time.Now()
	sg, err := c.CreateShardGroup("db0", "autogen", tmin)
	if err != nil {
		t.Fatal(err)
	} else if sg == nil {
		t.Fatalf("expected ShardGroup")
	}

	// This will create a shard group
	dur := sg.EndTime.Sub(sg.StartTime)
	tmax := tmin.Add(dur)
	if err := c.PrecreateShardGroups(tmin, tmax); err != nil {
		t.Fatal(err)
	}

	// This won't create a shard group, tmax2 is before tmax at timeline.
	dur = 24 * time.Hour
	tmax2 := tmin.Add(dur)
	if err := c.PrecreateShardGroups(tmin, tmax2); err != nil {
		t.Fatal(err)
	}

	groups, err := c.ShardGroupsByTimeRange("db0", "autogen", tmin, tmax)
	if err != nil {
		t.Fatal(err)
	} else if len(groups) != 2 {
		t.Fatalf("wrong number of shard groups: %d", len(groups))
	}

	//TODO use ShardsByTimeRange return ShardGroupInfo
	//how to construct cnosql.Sources
	//sources := cnosql.Sources{}
	//groups2, err := c.ShardsByTimeRange(sources,tmin,tmax)
	//if err != nil {
	//	t.Fatal(err)
	//} else if len(groups2) != 2 {
	//	t.Fatalf("wrong number of shard groups: %d", len(groups))
	//}

	sds := c.ShardIDs()
	if len(sds) != 2 {
		t.Fatalf("The length of the shard IDs should be 2")
	}

}

func TestMetaClient_CreateShardGroupIdempotent(t *testing.T) {
	t.Parallel()

	d, c := newClient()
	defer os.RemoveAll(d)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	// create a shard group.
	tmin := time.Now()
	sg, err := c.CreateShardGroup("db0", "autogen", tmin)
	if err != nil {
		t.Fatal(err)
	} else if sg == nil {
		t.Fatalf("expected ShardGroup")
	}

	i := c.Data().Index
	t.Log("index: ", i)

	// create the same shard group.
	sg, err = c.CreateShardGroup("db0", "autogen", tmin)
	if err != nil {
		t.Fatal(err)
	} else if sg == nil {
		t.Fatalf("expected ShardGroup")
	}

	t.Log("index: ", i)
	if got, exp := c.Data().Index, i; got != exp {
		t.Fatalf("PrecreateShardGroups failed: invalid index, got %d, exp %d", got, exp)
	}

	// make sure pre-creating is also idempotent
	// Test pre-creating shard groups.
	dur := sg.EndTime.Sub(sg.StartTime) + time.Nanosecond
	tmax := tmin.Add(dur)
	if err := c.PrecreateShardGroups(tmin, tmax); err != nil {
		t.Fatal(err)
	}
	i = c.Data().Index
	t.Log("index: ", i)
	if err := c.PrecreateShardGroups(tmin, tmax); err != nil {
		t.Fatal(err)
	}
	t.Log("index: ", i)
	if got, exp := c.Data().Index, i; got != exp {
		t.Fatalf("PrecreateShardGroups failed: invalid index, got %d, exp %d", got, exp)
	}
}

func TestMetaClient_PruneShardGroups(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	// This database is no use,just occupies the space.
	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	//we use this database for testing.
	if _, err := c.CreateDatabase("db1"); err != nil {
		t.Fatal(err)
	}

	duration := 1 * time.Hour
	replicaN := 1

	if _, err := c.CreateRetentionPolicy("db1", &meta.RetentionPolicySpec{
		Name:     "rp0",
		Duration: &duration,
		ReplicaN: &replicaN,
	}, true); err != nil {
		t.Fatal(err)
	}

	sg, err := c.CreateShardGroup("db1", "autogen", time.Now())
	if err != nil {
		t.Fatal(err)
	} else if sg == nil {
		t.Fatalf("expected ShardGroup")
	}

	sg, err = c.CreateShardGroup("db1", "autogen", time.Now().Add(15*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	} else if sg == nil {
		t.Fatalf("expected ShardGroup")
	}

	sg, err = c.CreateShardGroup("db1", "rp0", time.Now())
	if err != nil {
		t.Fatal(err)
	} else if sg == nil {
		t.Fatalf("expected ShardGroup")
	}

	expiration := time.Now().Add(-2 * 7 * 24 * time.Hour).Add(-1 * time.Hour)

	//modify the deleteAt time so that we can prune them.
	data := c.Data()
	data.Databases[1].RetentionPolicies[0].ShardGroups[0].DeletedAt = expiration
	data.Databases[1].RetentionPolicies[0].ShardGroups[1].DeletedAt = expiration

	if err := c.SetData(&data); err != nil {
		t.Fatal(err)
	}

	if err := c.PruneShardGroups(); err != nil {
		t.Fatal(err)
	}

	data = c.Data()
	rp, err := data.RetentionPolicy("db1", "autogen")
	if err != nil {
		t.Fatal(err)
	}
	if got, exp := len(rp.ShardGroups), 0; got != exp {
		t.Fatalf("failed to prune shard group. got: %d, exp: %d", got, exp)
	}

	rp, err = data.RetentionPolicy("db1", "rp0")
	if err != nil {
		t.Fatal(err)
	}
	if got, exp := len(rp.ShardGroups), 1; got != exp {
		t.Fatalf("failed to prune shard group. got: %d, exp: %d", got, exp)
	}
}

func TestMetaClient_TruncateShardGroups(t *testing.T) {

	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	if _, err := c.CreateDatabase("db0"); err != nil {
		t.Fatal(err)
	}

	// creat a shard group.
	t1 := time.Now()
	sg1, err := c.CreateShardGroup("db0", "autogen", t1)
	if err != nil {
		t.Fatal(err)
	} else if sg1 == nil {
		t.Fatalf("expected ShardGroup")
	}

	dur := time.Hour * 168
	t2 := t1.Add(dur)
	//create another shard group
	sg2, err := c.CreateShardGroup("db0", "autogen", t2)
	if err != nil {
		t.Fatal(err)
	} else if sg2 == nil {
		t.Fatalf("expected ShardGroup")
	}

	//Truncate now()
	err = c.TruncateShardGroups(t1)
	if err != nil {
		t.Fatal(err)
	}

	groups, err := c.ShardGroupsByTimeRange("db0", "autogen", sg1.StartTime, sg2.EndTime)
	if err != nil {
		t.Fatal(err)
	} else if groups[0].TruncatedAt != t1 {
		t.Fatalf("sg1's TruncatedAt should equal now()")
	} else if groups[1].TruncatedAt != groups[1].StartTime {
		t.Fatalf("sg1's TruncatedAt should equal shard start time")
	}

}

func TestMetaClient_DataNode(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	_, err := c.DataNode(0)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.DataNodes()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.CreateDataNode("", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.DataNodeByHTTPHost("")
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.DataNodeByTCPHost("")
	if err != nil {
		t.Fatal(err)
	}

	err = c.DeleteDataNode(0)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.MetaNodes()
	if err != nil {
		t.Fatal(err)
	}

	_ = c.MetaNodeByAddr("")

	_, err = c.CreateMetaNode("", "")
	if err != nil {
		t.Fatal(err)
	}

	err = c.DeleteMetaNode(0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMetaClient_PersistClusterIDAfterRestart(t *testing.T) {
	t.Parallel()

	cfg := newConfig()
	defer os.RemoveAll(cfg.Dir)

	c := meta.NewClient(cfg)
	if err := c.Open(); err != nil {
		t.Fatal(err)
	}
	id := c.ClusterID()
	if id == 0 {
		t.Fatal("cluster ID can't be zero")
	}

	c = meta.NewClient(cfg)
	if err := c.Open(); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	idAfter := c.ClusterID()
	if idAfter == 0 {
		t.Fatal("cluster ID can't be zero")
	} else if idAfter != id {
		t.Fatalf("cluster id not the same: %d, %d", idAfter, id)
	}
}

func TestMetaClient_Ping(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	err := c.Ping(true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMetaClient_AcquireLease(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	lease, err := c.AcquireLease("lease")
	if err != nil {
		return
	} else if lease.Name != "lease" {
		t.Fatalf("lease name error")
	}
}

func TestMetaClient_NodeID(t *testing.T) {
	t.Parallel()

	dir, c := newClient()
	defer os.RemoveAll(dir)
	defer c.Close()

	id := c.NodeID()

	if id != 0 {
		t.Fatalf("node ID is wrong")
	}
}

func newClient() (string, *meta.Client) {
	config := newConfig()
	c := meta.NewClient(config)
	if err := c.Open(); err != nil {
		panic(err)
	}
	return config.Dir, c
}

func newConfig() *meta.Config {
	cfg := meta.NewConfig()
	cfg.Dir = makeTempDir(2)
	return cfg
}

func makeTempDir(skip int) string {
	var pc, _, _, ok = runtime.Caller(skip)
	if !ok {
		panic("failed to get name of test function")
	}
	_, prefix := path.Split(runtime.FuncForPC(pc).Name())

	dir, err := ioutil.TempDir(os.TempDir(), prefix)
	if err != nil {
		panic(err)
	}
	return dir
}

func isAdmin(u meta.User) bool {
	return u.(*meta.UserInfo).Admin
}
