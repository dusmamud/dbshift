package mongo

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"github.com/dusmamud/dbshift/internal/core"
)

// Driver implements core.Driver for MongoDB Atlas and self-hosted MongoDB clusters.
type Driver struct {
	client   *mongo.Client
	db       *mongo.Database
	uri      string
	dbName   string
	version  string
	mu       sync.RWMutex
}

// NewDriver creates an uninitialized Mongo driver.
func NewDriver() *Driver {
	return &Driver{}
}

// Engine returns core.EngineMongo.
func (d *Driver) Engine() core.EngineType {
	return core.EngineMongo
}

// DialectInfo returns MongoDB version info.
func (d *Driver) DialectInfo() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.version != "" {
		return "MongoDB Atlas (" + d.version + ")"
	}
	return "MongoDB Atlas / DocumentDB"
}

// Connect connects to MongoDB using the provided connection string.
func (d *Driver) Connect(ctx context.Context, uri string) error {
	d.uri = uri

	// Extract database name from URI or default to "admin"
	dbName := extractMongoDbName(uri)
	d.dbName = dbName

	opts := options.Client().ApplyURI(uri)
	opts.SetMaxPoolSize(20)
	opts.SetMinPoolSize(2)

	client, err := mongo.Connect(opts)
	if err != nil {
		return core.NewError(core.EngineMongo, "Connect", "failed to connect to MongoDB", err)
	}

	d.client = client
	d.db = client.Database(dbName)

	// Fetch build info / version
	var buildInfo bson.M
	if err := d.db.RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&buildInfo); err == nil {
		if v, ok := buildInfo["version"].(string); ok {
			d.mu.Lock()
			d.version = "v" + v
			d.mu.Unlock()
		}
	}

	return nil
}

// Ping verifies MongoDB connection.
func (d *Driver) Ping(ctx context.Context) error {
	if d.client == nil {
		return core.NewError(core.EngineMongo, "Ping", "client is not connected", nil)
	}
	if err := d.client.Ping(ctx, nil); err != nil {
		return core.NewError(core.EngineMongo, "Ping", "ping failed", err)
	}
	return nil
}

// Inspect lists collections and document counts.
func (d *Driver) Inspect(ctx context.Context) ([]core.CollectionMeta, error) {
	if d.db == nil {
		return nil, core.NewError(core.EngineMongo, "Inspect", "not connected", nil)
	}

	collNames, err := d.db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, core.NewError(core.EngineMongo, "Inspect", "failed to list collections", err)
	}

	var results []core.CollectionMeta

	for _, name := range collNames {
		// Ignore internal system collections
		if strings.HasPrefix(name, "system.") {
			continue
		}

		coll := d.db.Collection(name)

		// Get document count
		count, err := coll.EstimatedDocumentCount(ctx)
		if err != nil {
			count, _ = coll.CountDocuments(ctx, bson.D{})
		}

		meta := core.CollectionMeta{
			Name:          name,
			EstimatedRows: count,
		}

		// Inspect indexes
		cur, err := coll.Indexes().List(ctx)
		if err == nil {
			var indexes []core.IndexMeta
			for cur.Next(ctx) {
				var idxDoc bson.M
				if err := cur.Decode(&idxDoc); err == nil {
					idxName, _ := idxDoc["name"].(string)
					// Skip default _id_ index
					if idxName == "_id_" {
						continue
					}
					indexes = append(indexes, core.IndexMeta{
						Name:      idxName,
						ExtraOpts: idxDoc,
					})
				}
			}
			cur.Close(ctx)
			meta.Indexes = indexes
		}

		results = append(results, meta)
	}

	return results, nil
}

// CreateSchema creates collections and indexes on target MongoDB.
func (d *Driver) CreateSchema(ctx context.Context, meta core.CollectionMeta, dropExisting bool) error {
	if d.db == nil {
		return core.NewError(core.EngineMongo, "CreateSchema", "not connected", nil)
	}

	coll := d.db.Collection(meta.Name)

	if dropExisting {
		_ = coll.Drop(ctx)
	}

	// Recreate indexes
	if len(meta.Indexes) > 0 {
		var models []mongo.IndexModel
		for _, idx := range meta.Indexes {
			if keyDoc, ok := idx.ExtraOpts["key"].(bson.M); ok {
				var keys bson.D
				for k, v := range keyDoc {
					keys = append(keys, bson.E{Key: k, Value: v})
				}
				idxOpts := options.Index()
				if idx.Name != "" {
					idxOpts.SetName(idx.Name)
				}
				if u, ok := idx.ExtraOpts["unique"].(bool); ok && u {
					idxOpts.SetUnique(true)
				}
				models = append(models, mongo.IndexModel{
					Keys:    keys,
					Options: idxOpts,
				})
			}
		}

		if len(models) > 0 {
			_, _ = coll.Indexes().CreateMany(ctx, models)
		}
	}

	return nil
}

// StreamCopy streams documents from source collection to target collection using BulkWrite.
func (d *Driver) StreamCopy(
	ctx context.Context,
	target core.Driver,
	meta core.CollectionMeta,
	batchSize int,
	progress chan<- core.ProgressEvent,
) error {
	targetMongo, ok := target.(*Driver)
	if !ok || targetMongo.db == nil {
		return core.NewError(core.EngineMongo, "StreamCopy", "target is not a valid Mongo driver", nil)
	}

	if batchSize <= 0 {
		batchSize = 2500
	}

	srcColl := d.db.Collection(meta.Name)
	tgtColl := targetMongo.db.Collection(meta.Name)

	startTime := time.Now()
	totalEstimated := meta.EstimatedRows
	var totalCopied int64

	// Emit initial progress
	if progress != nil {
		progress <- core.ProgressEvent{
			CollectionName: meta.Name,
			TotalRows:      totalEstimated,
			CopiedRows:     0,
			SpeedRowsPerS:  0,
		}
	}

	cur, err := srcColl.Find(ctx, bson.D{}, options.Find().SetBatchSize(int32(batchSize)))
	if err != nil {
		return core.NewError(core.EngineMongo, "StreamCopy", "failed to open find cursor", err)
	}
	defer cur.Close(ctx)

	writeModels := make([]mongo.WriteModel, 0, batchSize)

	flushBatch := func() error {
		if len(writeModels) == 0 {
			return nil
		}

		bulkOpts := options.BulkWrite().SetOrdered(false)
		_, err := tgtColl.BulkWrite(ctx, writeModels, bulkOpts)
		if err != nil {
			// Check if duplicate key error; ignore duplicates if unordered
			if !strings.Contains(err.Error(), "E11000") {
				return core.NewError(core.EngineMongo, "StreamCopy", "bulk write error", err)
			}
		}

		totalCopied += int64(len(writeModels))
		elapsed := time.Since(startTime).Seconds()
		speed := 0.0
		if elapsed > 0 {
			speed = float64(totalCopied) / elapsed
		}

		if progress != nil {
			progress <- core.ProgressEvent{
				CollectionName: meta.Name,
				TotalRows:      totalEstimated,
				CopiedRows:     totalCopied,
				SpeedRowsPerS:  speed,
			}
		}

		writeModels = writeModels[:0]
		return nil
	}

	for cur.Next(ctx) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var doc bson.Raw
		if err := cur.Decode(&doc); err != nil {
			return core.NewError(core.EngineMongo, "StreamCopy", "decode error", err)
		}

		writeModels = append(writeModels, mongo.NewInsertOneModel().SetDocument(doc))

		if len(writeModels) >= batchSize {
			if err := flushBatch(); err != nil {
				return err
			}
		}
	}

	if err := cur.Err(); err != nil {
		return core.NewError(core.EngineMongo, "StreamCopy", "cursor error", err)
	}

	if err := flushBatch(); err != nil {
		return err
	}

	elapsed := time.Since(startTime).Seconds()
	speed := 0.0
	if elapsed > 0 {
		speed = float64(totalCopied) / elapsed
	}

	if progress != nil {
		progress <- core.ProgressEvent{
			CollectionName: meta.Name,
			TotalRows:      totalCopied,
			CopiedRows:     totalCopied,
			SpeedRowsPerS:  speed,
			IsCompleted:    true,
		}
	}

	return nil
}

// PostMigrationFixups is a no-op for Mongo.
func (d *Driver) PostMigrationFixups(ctx context.Context, collections []core.CollectionMeta) error {
	return nil
}

// VerifyCounts returns accurate document count.
func (d *Driver) VerifyCounts(ctx context.Context, collectionName string) (int64, error) {
	if d.db == nil {
		return 0, core.NewError(core.EngineMongo, "VerifyCounts", "not connected", nil)
	}
	return d.db.Collection(collectionName).CountDocuments(ctx, bson.D{})
}

// Close closes MongoDB connection.
func (d *Driver) Close(ctx context.Context) error {
	if d.client != nil {
		return d.client.Disconnect(ctx)
	}
	return nil
}

func extractMongoDbName(uri string) string {
	// e.g. mongodb+srv://username:password@example.com/myDatabase?retryWrites=true
	clean := uri
	if idx := strings.Index(clean, "?"); idx != -1 {
		clean = clean[:idx]
	}
	parts := strings.Split(clean, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if last != "" && !strings.Contains(last, "@") && !strings.Contains(last, ":") {
			return last
		}
	}
	return "test"
}
