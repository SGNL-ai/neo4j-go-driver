package bolt

import (
	"context"
	"github.com/SGNL-ai/neo4j-go-driver/v5/neo4j/db"
)

type responseHandler struct {
	onSuccess func(*success)
	onRecord  func(*db.Record)
	onFailure func(context.Context, *db.Neo4jError)
	onUnknown func(any)
	onIgnored func(*ignored)
}

func onSuccessNoOp(*success) {}
func onIgnoredNoOp(*ignored) {}
