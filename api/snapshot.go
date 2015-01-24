package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/smira/aptly/deb"
)

// GET /api/snapshots
func apiSnapshotsList(c *gin.Context) {
	SortMethodString := c.Request.URL.Query().Get("sort")

	collection := context.CollectionFactory().SnapshotCollection()
	collection.RLock()
	defer collection.RUnlock()

	if SortMethodString == "" {
		SortMethodString = "name"
	}

	result := []*deb.Snapshot{}
	collection.ForEachSorted(SortMethodString, func(snapshot *deb.Snapshot) error {
		result = append(result, snapshot)
		return nil
	})

	c.JSON(200, result)
}

// POST /api/mirrors/:name/snapshots/
func apiSnapshotsCreateFromMirror(c *gin.Context) {
	var (
		err      error
		repo     *deb.RemoteRepo
		snapshot *deb.Snapshot
	)

	var b struct {
		Name        string `binding:"required"`
		Description string
	}

	if !c.Bind(&b) {
		return
	}

	collection := context.CollectionFactory().RemoteRepoCollection()
	collection.Lock()
	defer collection.Unlock()

	snapshotCollection := context.CollectionFactory().SnapshotCollection()
	snapshotCollection.Lock()
	defer snapshotCollection.Unlock()

	repo, err = collection.ByName(c.Params.ByName("name"))
	if err != nil {
		c.Fail(404, err)
		return
	}

	err = repo.CheckLock()
	if err != nil {
		c.Fail(409, err)
		return
	}

	err = collection.LoadComplete(repo)
	if err != nil {
		c.Fail(500, err)
		return
	}

	snapshot, err = deb.NewSnapshotFromRepository(b.Name, repo)
	if err != nil {
		c.Fail(400, err)
		return
	}

	if b.Description != "" {
		snapshot.Description = b.Description
	}

	err = snapshotCollection.Add(snapshot)
	if err != nil {
		c.Fail(500, err)
		return
	}

	c.JSON(201, snapshot)
}

// POST /api/snapshots
func apiSnapshotsCreate(c *gin.Context) {
	var (
		err      error
		snapshot *deb.Snapshot
	)

	var b struct {
		Name        string `binding:"required"`
		Description string
		SourceIDs   []string
		PackageRefs []string
	}

	if !c.Bind(&b) {
		return
	}

	if b.Description == "" {
		if len(b.SourceIDs)+len(b.PackageRefs) == 0 {
			b.Description = "Created as empty"
		}
	}

	snapshotCollection := context.CollectionFactory().SnapshotCollection()
	snapshotCollection.Lock()
	defer snapshotCollection.Unlock()

	sources := make([]*deb.Snapshot, len(b.SourceIDs))

	for i := 0; i < len(b.SourceIDs); i++ {
		sources[i], err = snapshotCollection.ByUUID(b.SourceIDs[i])
		if err != nil {
			c.Fail(404, err)
			return
		}

		err = snapshotCollection.LoadComplete(sources[i])
		if err != nil {
			c.Fail(500, err)
			return
		}
	}

	packageRefs := make([][]byte, len(b.PackageRefs))
	for i, ref := range b.PackageRefs {
		packageRefs[i] = []byte(ref)
	}

	packageRefList := &deb.PackageRefList{packageRefs}
	snapshot = deb.NewSnapshotFromRefList(b.Name, sources, packageRefList, b.Description)

	err = snapshotCollection.Add(snapshot)
	if err != nil {
		c.Fail(500, err)
		return
	}

	c.JSON(201, snapshot)
}

// POST /api/repos/:name/snapshots/:snapname
func apiSnapshotsCreateFromRepository(c *gin.Context) {
	var (
		err      error
		repo     *deb.LocalRepo
		snapshot *deb.Snapshot
	)

	var b struct {
		Name        string `binding:"required"`
		Description string
	}

	if !c.Bind(&b) {
		return
	}

	collection := context.CollectionFactory().LocalRepoCollection()
	collection.Lock()
	defer collection.Unlock()

	snapshotCollection := context.CollectionFactory().SnapshotCollection()
	snapshotCollection.Lock()
	defer snapshotCollection.Unlock()

	repo, err = collection.ByName(c.Params.ByName("name"))
	if err != nil {
		c.Fail(404, err)
		return
	}

	err = collection.LoadComplete(repo)
	if err != nil {
		c.Fail(500, err)
		return
	}

	snapshot, err = deb.NewSnapshotFromLocalRepo(b.Name, repo)
	if err != nil {
		c.Fail(400, err)
		return
	}

	if b.Description != "" {
		snapshot.Description = b.Description
	}

	err = snapshotCollection.Add(snapshot)
	if err != nil {
		c.Fail(500, err)
		return
	}

	c.JSON(201, snapshot)
}

// PUT /api/snapshots/:name
func apiSnapshotsUpdate(c *gin.Context) {
	var (
		err      error
		snapshot *deb.Snapshot
	)

	var b struct {
		Name        string
		Description string
	}

	if !c.Bind(&b) {
		return
	}

	collection := context.CollectionFactory().SnapshotCollection()
	collection.Lock()
	defer collection.Unlock()

	snapshot, err = context.CollectionFactory().SnapshotCollection().ByName(c.Params.ByName("name"))
	if err != nil {
		c.Fail(404, err)
		return
	}

	_, err = context.CollectionFactory().SnapshotCollection().ByName(b.Name)
	if err == nil {
		c.Fail(409, fmt.Errorf("unable to rename: snapshot %s already exists", b.Name))
		return
	}

	if b.Name != "" {
		snapshot.Name = b.Name
	}

	if b.Description != "" {
		snapshot.Description = b.Description
	}

	err = context.CollectionFactory().SnapshotCollection().Update(snapshot)
	if err != nil {
		c.Fail(403, err)
		return
	}

	c.JSON(200, snapshot)
}

// GET /api/snapshots/:name
func apiSnapshotsShow(c *gin.Context) {
	collection := context.CollectionFactory().SnapshotCollection()
	collection.RLock()
	defer collection.RUnlock()

	snapshot, err := collection.ByName(c.Params.ByName("name"))
	if err != nil {
		c.Fail(404, err)
		return
	}

	err = collection.LoadComplete(snapshot)
	if err != nil {
		c.Fail(500, err)
		return
	}

	c.JSON(200, snapshot)
}

// DELETE /api/snapshots/:name
func apiSnapshotsDrop(c *gin.Context) {
	name := c.Params.ByName("name")
	force := c.Request.URL.Query().Get("force") == "1"

	collection := context.CollectionFactory().LocalRepoCollection()
	collection.Lock()
	defer collection.Unlock()

	snapshotCollection := context.CollectionFactory().SnapshotCollection()
	snapshotCollection.RLock()
	defer snapshotCollection.RUnlock()

	publishedCollection := context.CollectionFactory().PublishedRepoCollection()
	publishedCollection.RLock()
	defer publishedCollection.RUnlock()

	snapshot, err := snapshotCollection.ByName(name)
	if err != nil {
		c.Fail(404, err)
		return
	}

	published := publishedCollection.BySnapshot(snapshot)

	if len(published) > 0 {
		for _, repo := range published {
			err = publishedCollection.LoadComplete(repo, context.CollectionFactory())
			if err != nil {
				c.Fail(500, err)
				return
			}
		}

		c.Fail(409, fmt.Errorf("unable to drop: snapshot is published"))
		return
	}

	if !force {
		snapshots := snapshotCollection.BySnapshotSource(snapshot)
		if len(snapshots) > 0 {
			c.Fail(409, fmt.Errorf("won't delete snapshot that was used as source for other snapshots, use ?force=1 to override"))
			return
		}
	}

	err = context.CollectionFactory().SnapshotCollection().Drop(snapshot)
	if err != nil {
		c.Fail(500, err)
		return
	}

	c.JSON(200, gin.H{})
}

// GET /api/snapshots/:name/diff/:withSnapshot
func apiSnapshotsDiff(c *gin.Context) {
	onlyMatching := c.Request.URL.Query().Get("onlyMatching") == "1"

	collection := context.CollectionFactory().SnapshotCollection()
	collection.RLock()
	defer collection.RUnlock()

	snapshotA, err := collection.ByName(c.Params.ByName("name"))
	if err != nil {
		c.Fail(404, err)
		return
	}

	snapshotB, err := collection.ByName(c.Params.ByName("withSnapshot"))
	if err != nil {
		c.Fail(404, err)
		return
	}

	err = context.CollectionFactory().SnapshotCollection().LoadComplete(snapshotA)
	if err != nil {
		c.Fail(500, err)
		return
	}

	err = context.CollectionFactory().SnapshotCollection().LoadComplete(snapshotB)
	if err != nil {
		c.Fail(500, err)
		return
	}

	// Calculate diff
	diff, err := snapshotA.RefList().Diff(snapshotB.RefList(), context.CollectionFactory().PackageCollection())
	if err != nil {
		c.Fail(500, err)
		return
	}

	result := []deb.PackageDiff{}

	for _, pdiff := range diff {
		if onlyMatching && (pdiff.Left == nil || pdiff.Right == nil) {
			continue
		}

		result = append(result, pdiff)
	}

	c.JSON(200, result)
}

// GET /api/snapshots/:name/packages
func apiSnapshotsSearchPackages(c *gin.Context) {
	collection := context.CollectionFactory().SnapshotCollection()
	collection.RLock()
	defer collection.RUnlock()

	snapshot, err := collection.ByName(c.Params.ByName("name"))
	if err != nil {
		c.Fail(404, err)
		return
	}

	err = collection.LoadComplete(snapshot)
	if err != nil {
		c.Fail(500, err)
		return
	}

	showPackages(c, snapshot.RefList())
}
