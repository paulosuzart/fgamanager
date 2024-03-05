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
	db         *sqlx.DB
	Repository TupleRepository
)

type TupleRepository interface {
	CountTuples(filter *Filter) int
	GetMarkedForDeletion() []Tuple
	ApplyChange(change openfga.TupleChange)
	Prune() int
}

type SqlxRepository struct {
	TupleRepository
	_db *sqlx.DB
}

func (r *SqlxRepository) CountTuples(filter *Filter) int {
	return countTuples(filter)
}

func (r *SqlxRepository) GetMarkedForDeletion() []Tuple {
	return getMarkedForDeletion()
}

func (r *SqlxRepository) ApplyChange(change openfga.TupleChange) {
	applyChange(change)
}

func (r *SqlxRepository) Prune() int {
	affectedRows := 0
	err := Transact(func() {
		rows, err := db.Queryx("select tuple_key from pending_actions where action = 'S'")
		if err != nil {
			log.Panic("Failed to query stale data")
		}
		var ids []string
		for rows.Next() {
			var tupleKey string
			err = rows.Scan(&tupleKey)
			if err != nil {
				log.Printf("Failed to scan row %v", err)
			}
			ids = append(ids, tupleKey)
		}
		_ = rows.Close()

		query, args, _ := sqlx.In(`delete from tuples where tuple_key in (?)`, ids)
		query = db.Rebind(query)
		db.MustExec(query, args...)
		query, args, _ = sqlx.In(`delete from pending_actions where tuple_key in (?)`, ids)
		query = db.Rebind(query)
		db.MustExec(query, args...)
		affectedRows = len(ids)
	})
	if err != nil {
		log.Printf("Failed to transact prune")
		return 0
	}
	return affectedRows
}

func newRepository() TupleRepository {
	var repo TupleRepository
	repo = &SqlxRepository{}
	return repo
}

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

func setupDb(dataSource string) {
	_db, err := sqlx.Open("sqlite3", dataSource)
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
		
		CREATE INDEX IF NOT EXISTS idx_tuple_content on tuples(user_type, user_id, relation, object_id, object_type);
		
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
	db.MustExec(sts)
	Repository = newRepository()
	log.Printf("Finished db setup")
}
func SetupDb() {
	setupDb("fga.db")
}

// applyChange Takes a tuple change straight from the API
func applyChange(change openfga.TupleChange) {
	userType, userId := splitTypePair(change.TupleKey.GetUser())
	relation := change.TupleKey.GetRelation()
	objectType, objectId := splitTypePair(change.TupleKey.GetObject())
	tupleKey := fmt.Sprintf("%s %s %s",
		change.GetTupleKey().User,
		change.GetTupleKey().Relation,
		change.GetTupleKey().Object)

	// ensures whatever existing action is cleaned up
	db.MustExec("delete from pending_actions where tuple_key = ?", tupleKey)

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

type Filter struct {
	Search     *string
	UserType   *string
	Relation   *string
	ObjectType *string
}

func (f *Filter) isSet() bool {
	return f.Search != nil || f.UserType != nil || f.Relation != nil || f.ObjectType != nil
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
	*Tuple
	*PendingAction
}

// LoadResult represents the last page load
type LoadResult struct {
	// the first item loaded row number
	lowerBound int
	// the last item loaded row number
	upperBound int
	// the tuple content itself
	Res []TuplePendingAction
	// whatever was the filter user, it's returned
	Filter *Filter
	total  int
}

func (l *LoadResult) GetTotal() int {
	return l.total
}

func (l *LoadResult) GetLowerBound() int {
	return l.lowerBound
}

func (l *LoadResult) GetUpperBound() int {
	return l.upperBound
}

func Load(offset int, filter *Filter) *LoadResult {

	var params = map[string]interface{}{
		"offset": offset,
	}

	var whereClauses []string
	if filter != nil && filter.Search != nil && len(strings.TrimSpace(*filter.Search)) > 3 {
		whereClauses = append(whereClauses, "tuples.tuple_key like :query\n")
		params["query"] = filter.Search
	}
	if filter != nil && filter.UserType != nil {
		whereClauses = append(whereClauses, "tuples.user_type = :userType\n")
		params["userType"] = filter.UserType
	}
	if filter != nil && filter.Relation != nil {
		whereClauses = append(whereClauses, "tuples.relation = :relation\n")
		params["relation"] = filter.Relation
	}
	if filter != nil && filter.ObjectType != nil {
		whereClauses = append(whereClauses, "tuples.object_type = :objectType\n")
		params["objectType"] = filter.ObjectType
	}

	finalWhere := strings.Join(whereClauses[:], " and ")
	if finalWhere != "" {
		finalWhere = " where " + finalWhere
	}

	selectClause := fmt.Sprintf(`
			select tuples.*, p.action from (select *, row_number() over (order by timestamp desc) as row_number from tuples %v) tuples
			         left join pending_actions p on tuples.tuple_key = p.tuple_key 
			where row_number >= :offset and row_number <= :offset + 200
			`, finalWhere)

	log.Printf("Load Query: %v\noffset: %v", selectClause, offset)
	rows, err := db.NamedQuery(selectClause, params)
	if err != nil {
		log.Fatal(err)
	}

	var res []TuplePendingAction
	for rows.Next() {
		var p TuplePendingAction
		err = rows.StructScan(&p)
		res = append(res, p)
	}
	err = rows.Close()
	if err != nil {
		log.Panic(err)
	}
	if len(res) == 0 {
		return nil
	}

	return &LoadResult{
		lowerBound: res[0].Row,
		upperBound: res[len(res)-1].Row,
		Res:        res,
		Filter:     filter,
		total:      Repository.CountTuples(filter),
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

func countTuples(filter *Filter) int {
	selectClause := "select count(*) as count from tuples"
	var params = make(map[string]interface{})
	if filter != nil && filter.isSet() {
		selectClause = fmt.Sprintf("%v where", selectClause)
	}
	if filter != nil && filter.Search != nil && len(strings.TrimSpace(*filter.Search)) > 3 {
		selectClause = fmt.Sprintf("%s tuples.tuple_key like :query\n", selectClause)
		params["query"] = filter.Search
	}
	if filter != nil && filter.UserType != nil {
		if len(params) > 0 {
			selectClause = fmt.Sprintf("%s and ", selectClause)
		}
		selectClause = fmt.Sprintf("%s tuples.user_type = :userType\n", selectClause)
		params["userType"] = filter.UserType
	}
	if filter != nil && filter.Relation != nil {
		if len(params) > 0 {
			selectClause = fmt.Sprintf("%s and ", selectClause)
		}
		selectClause = fmt.Sprintf("%s tuples.relation = :relation\n", selectClause)
		params["relation"] = filter.Relation
	}
	if filter != nil && filter.ObjectType != nil {
		if len(params) > 0 {
			selectClause = fmt.Sprintf("%s and ", selectClause)
		}
		selectClause = fmt.Sprintf("%s tuples.object_type = :objectType", selectClause)
		params["objectType"] = filter.ObjectType
	}

	log.Printf("Count query '%v'", selectClause)

	res, err := db.NamedQuery(selectClause, params)
	if err != nil {
		log.Fatal(err)
		return 0
	}
	res.Next()
	var count int
	err = res.Scan(&count)
	if err != nil {
		return 0
	}
	err = res.Close()
	if err != nil {
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

func MarkStale(tupleKey string) {
	sql := `insert into pending_actions (tuple_key, action) values (?, 'S') 
            on conflict do update set action = 'S'`
	_, err := db.Exec(sql, tupleKey)

	if err != nil {
		log.Printf("Failed making for deletion %v", err.Error())
	}
}

func getTypes(typeToCount string) []string {
	result, err := db.Query(fmt.Sprintf("select distinct %v from tuples order by 1", typeToCount))
	if err != nil {
		log.Printf("Failed to get user Types %v", err.Error())
		return []string{"ERROR"}
	}
	var userTypes []string
	for result.Next() {
		var userType string
		err := result.Scan(&userType)
		if err != nil {
			log.Panic(err)
		}
		userTypes = append(userTypes, userType)
	}
	err = result.Close()
	if err != nil {
		return nil
	}
	return userTypes
}

func GetUserTypes() []string {
	return getTypes("user_type")
}

func GetRelations() []string {
	return getTypes("relation")
}

func GetObjectTypes() []string {
	return getTypes("object_type")
}

func getMarkedForDeletion() []Tuple {
	sql := `select tuples.* from tuples join pending_actions on pending_actions.tuple_key = tuples.tuple_key and
		pending_actions.action = 'D' limit 10
	`
	rows, err := db.Queryx(sql)
	if err != nil {
		log.Printf("Failed to fetch marked for deletion")
		return nil
	}
	var results []Tuple
	for rows.Next() {
		var tuple Tuple
		err := rows.StructScan(&tuple)
		if err != nil {
			log.Printf("Failed to Scan row %v", err)
			return nil
		}
		results = append(results, tuple)
	}
	_ = rows.Close()
	return results
}
