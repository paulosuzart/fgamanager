package db

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	openfga "github.com/openfga/go-sdk"
	"log"
	"strings"
	"time"
)

var (
	db *sqlx.DB
)

// Transact keeps it simple and executes the passed function
func Transact(f func()) error {
	tx := db.MustBegin()
	f()
	err := tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func SetupDb() {
	_db, err := sqlx.Open("sqlite3", "fga.db")
	if err != nil {
		log.Panic(err)
	}
	db = _db
	sts := `
		CREATE TABLE IF NOT EXISTS tuples(
			tuple_key text not null primary key,
		    user_type text not null, 
			user_id text not null,
			relation text not null,
			object_type text not null, 
			object_id text not null,
			timestamp timestamp);
		
		CREATE TABLE IF NOT EXISTS pending_actions (
		    tuple_key text not null primary key,
			action text not null);

		CREATE TABLE IF NOT EXISTS connections (
		    api_url text not null,
		    store_id text not null primary key ,
		    continuation_token text,
		    last_sync timestamp
		)
	`

	_, err = db.Exec(sts)

	if err != nil {
		log.Fatal(err)
	}

}

// ApplyChange Takes a tuple change straight from the API
func ApplyChange(change openfga.TupleChange) {
	userType, userId := splitTypePair(change.TupleKey.GetUser())
	relation := change.TupleKey.GetRelation()
	objectType, objectId := splitTypePair(change.TupleKey.GetObject())
	tupleKey := fmt.Sprintf("%s %s %s", change.GetTupleKey().User,
		change.GetTupleKey().Relation,
		change.GetTupleKey().Object)

	if change.Operation == openfga.WRITE {
		sql := `insert into tuples (
                    tuple_key,
                    user_type,
                    user_id,
                    relation,
                    object_type,
                    object_id,
                    timestamp ) values (:tuple_key,
                                        :user_type,
                                        :user_id,
                                        :relation,
                                        :object_type,
                                        :object_id,
                                        :timestamp) on conflict do update set timestamp = :timestamp`
		timestamp := change.GetTimestamp()
		_, err := db.NamedExec(sql, map[string]interface{}{
			"tuple_key":   tupleKey,
			"user_type":   userType,
			"user_id":     userId,
			"relation":    relation,
			"object_type": objectType,
			"object_id":   objectId,
			"timestamp":   timestamp,
		})

		if err != nil {
			log.Fatal(err)
		}
	} else if change.Operation == openfga.DELETE {
		sql := `delete from tuples 
	     		  where 
				   tuple_key = :tuple_key
				   `
		_, err := db.NamedExec(sql, map[string]interface{}{
			"tuple_key": tupleKey,
		})
		if err != nil {
			log.Fatal(err)
		}
	}
}

type Connection struct {
	ApiUrl            string    `db:"api_url"`
	StoreId           string    `db:"store_id"`
	ContinuationToken string    `db:"continuation_token"`
	LastSync          time.Time `db:"last_sync"`
}

func UpsertConnection(connection Connection) {
	_, err := db.NamedExec(`
			insert into connections (api_url, store_id, continuation_token, last_sync) 
				values (:api_url, :store_id, :continuation_token, :last_sync) on conflict do update 
			set continuation_token = :continuation_token, 
			    last_sync = :last_sync
		`, &connection)
	if err != nil {
		log.Fatal(err)
	}
}

func splitTypePair(typePair string) (string, string) {
	split := strings.Split(typePair, ":")
	return split[0], split[1]
}

func Close() {
	if db == nil {
		log.Fatal("Db close called but was not defined")
	}
	err := db.Close()
	if err != nil {
		log.Panic(err)
	}
}

type Tuple struct {
	TupleKey   string    `db:"tuple_key"`
	UserType   string    `db:"user_type"`
	UserId     string    `db:"user_id"`
	Relation   string    `db:"relation"`
	ObjectType string    `db:"object_type"`
	ObjectId   string    `db:"object_id"`
	Timestamp  time.Time `db:"timestamp"`
	Row        int       `db:"row_number"`
}

type PendingAction struct {
	Action string `db:"action"`
}

type TuplePendingAction struct {
	*Tuple         "db:tuples"
	*PendingAction "db:pending_actions"
}

type LoadResult struct {
	LowerBound int
	UpperBound int
	Res        []TuplePendingAction
	Query      string
}

func Load(offset int, query *string) *LoadResult {
	selectClause := `
			select tuples.*, p.action from (select *, row_number() over (order by timestamp desc) as row_number from tuples) tuples
			         left join pending_actions p on tuples.tuple_key = p.tuple_key 
			where row_number >= :offset and row_number <= :offset + 200
			`

	if query != nil && len(strings.TrimSpace(*query)) > 3 {
		selectClause = fmt.Sprintf("%s and tuples.tuple_key like :query", selectClause)
	}
	log.Printf("Load Query: %v", selectClause)
	rows, err := db.NamedQuery(selectClause, map[string]interface{}{
		"offset": offset,
		"query":  query,
	})
	if err != nil {
		log.Fatal(err)
	}

	var res []TuplePendingAction
	for rows.Next() {
		var p TuplePendingAction
		err = rows.StructScan(&p)
		res = append(res, p)
	}
	if len(res) == 0 {
		return nil
	}

	var finalQuery string
	if query != nil {
		finalQuery = *query
	} else {
		finalQuery = ""
	}
	return &LoadResult{
		LowerBound: res[0].Row,
		UpperBound: res[len(res)-1].Row,
		Res:        res,
		Query:      finalQuery,
	}
}

func GetContinuationToken(apiUrl, storeId string) *string {
	var token string
	err := db.Get(&token, `select continuation_token from connections 
                          where api_url = ? and store_id = ?`, apiUrl, storeId)

	if err != nil {
		return nil
	}
	return &token
}

func CountTuples(query *string) int {
	var count int
	selectClause := "select count(*) as count from tuples"
	if query != nil && len(strings.TrimSpace(*query)) > 3 {
		selectClause = fmt.Sprintf("%s where tuples.tuple_key like ?\n", selectClause)
	}
	log.Printf("Count query %v", selectClause)
	err := db.Get(&count, selectClause, query)
	if err != nil {
		log.Fatal(err)
		return 0
	}
	return count
}

func MarkDeletion(tupleKey string) {
	sql := `insert into pending_actions (tuple_key, action) values (?, 'D') 
            on conflict do nothing `
	_, err := db.Exec(sql, tupleKey)

	if err != nil {
		log.Printf("Failed making for deletion %v", err.Error())
	}
}

func GetUserTypes() []string {
	result, err := db.Query("select distinct user_type from tuples")
	if err != nil {
		log.Printf("Failed to get user Types %v", err.Error())
		return []string{"ERROR"}
	}
	var userTypes []string
	for result.Next() {
		var userType string
		result.Scan(&userType)
		userTypes = append(userTypes, userType)
	}
	return userTypes
}
