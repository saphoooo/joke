package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	redigotrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gomodule/redigo"
	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
	gormtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/jinzhu/gorm"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func main() {
	tracer.Start(tracer.WithServiceName("jokeback"))
	defer tracer.Stop()

	r := muxtrace.NewRouter(muxtrace.WithServiceName("jokeback"))
	r.HandleFunc("/api/jokes", jokeHandler).Methods("POST")
	log.Print("Start listening on :7000...")
	err := http.ListenAndServe(":7000", r)
	if err != nil {
		log.Panic().Msg(err.Error())
	}
}

func jokeHandler(w http.ResponseWriter, r *http.Request) {

	span, _ := tracer.StartSpanFromContext(r.Context(), "func.jokeHandler")
	defer span.Finish()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error().Msg(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "oops something wrong hapenned...")
		return
	}
	log.Trace().Msg("body: " + string(body))

	msg := message{}
	err = json.Unmarshal(body, &msg)
	if err != nil {
		log.Error().Msg(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "oops something wrong hapenned...")
		return
	}

	var todaysJokeContent string

	pool := &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redigotrace.Dial("tcp", "redis-master:6379",
				redigotrace.WithServiceName("redis"),
			)
		},
	}

	if todaysJokeContent, ok := checkCache(msg.Weekday, pool, span.Context()); ok {
		log.Trace().Str("todaysJokeContent", todaysJokeContent).Bool("ok", ok).Msg("entering checkCache loop")
		log.Trace().Msg("message from cache ok")
		todaysJoke := joke{Joke: todaysJokeContent}
		req, err := json.Marshal(todaysJoke)
		if err != nil {
			log.Panic().Msg(err.Error())
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, string(req))
		return
	}

	log.Debug().Msg("unable to retrieve joke from cache, looking for database")

	todaysJokeContent, err = queryJoke(msg.Weekday, span.Context())
	if err != nil {
		log.Error().Msg(err.Error())
	}

	todaysJoke := joke{Joke: todaysJokeContent}
	req, err := json.Marshal(todaysJoke)
	if err != nil {
		log.Panic().Msg(err.Error())
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(req))

	log.Trace().Msg("setting up the cache")
	updateCache(msg.Weekday, todaysJokeContent, pool, span.Context())
}

func checkCache(day string, pool *redis.Pool, spanctx ddtrace.SpanContext) (string, bool) {
	conn := pool.Get()
	defer conn.Close()

	span, ctx := tracer.StartSpanFromContext(context.Background(), "checkCache", tracer.ChildOf(spanctx))
	defer span.Finish()

	log.Trace().Msg("connecting to Redis...")
	_, err := conn.Do("AUTH", os.Getenv("REDIS_PASSWORD"), ctx)
	if err != nil {
		log.Error().Msg("error during authentication")
		return "", false
	}

	log.Trace().Msg("checking if key exists...")
	exists, err := redis.Int(conn.Do("EXISTS", day, ctx))
	if err != nil {
		log.Error().Msg("error getting the key")
		return "", false
	}
	log.Trace().Int("exists", exists).Msg("current value of exists")
	if exists == 0 { // the key does not exist
		log.Debug().Msg("the key isn't set")
		return "", false
	}
	log.Trace().Msg("getting the value...")
	joke, err := redis.String(conn.Do("GET", day, ctx))
	if err != nil {
		log.Error().Msg("error getting the value from cache")
		return "", false
	}
	return joke, true
}

func updateCache(day, joke string, pool *redis.Pool, spanctx ddtrace.SpanContext) {
	conn := pool.Get()
	defer conn.Close()

	span, ctx := tracer.StartSpanFromContext(context.Background(), "updateCache", tracer.ChildOf(spanctx))
	defer span.Finish()

	log.Trace().Msg("connecting to Redis...")
	_, err := conn.Do("AUTH", os.Getenv("REDIS_PASSWORD"), ctx)
	if err != nil {
		log.Error().Msg("error during authentication")
	}

	log.Trace().Msg("setting up the key...")
	_, err = conn.Do("SET", day, joke, ctx)
	if err != nil {
		log.Error().Msg("error setting key")
	}
	// set expiry of 30 seconds
	log.Trace().Msg("setting up the expiry...")
	_, err = conn.Do("EXPIRE", day, 30, ctx)
	if err != nil {
		log.Error().Msg("error setting expiry")
	}
}

func queryJoke(day string, spanctx ddtrace.SpanContext) (joke string, err error) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "func.queryJoke", tracer.ChildOf(spanctx))
	defer span.Finish()

	psqlInfo := "host=postgresql port=5432 user=postgres password=datadog101 dbname=datadog sslmode=disable"
	sqltrace.Register("postgres", &pq.Driver{}, sqltrace.WithServiceName("postgresql"))
	db, err := gormtrace.Open("postgres", psqlInfo, gormtrace.WithServiceName("postgresql"))
	db = gormtrace.WithContext(ctx, db)
	// db, err := gormtrace.Open("postgres", "postgres://postgres:datadog101@postgresql.default.svc/datadog?sslmode=disable")
	if err != nil {
		log.Error().Msg("unable to connect to database")
	}
	defer db.Close()

	var jokes Jokes
	db.Where("DAY = ?", day).First(&jokes)
	log.Trace().Uint("ID", jokes.ID).Str("DAY", jokes.DAY).Str("JOKE", jokes.JOKE).Msg("unable to connect to database")
	return jokes.JOKE, nil

}

type message struct {
	Weekday string `json:"weekday"`
}

type joke struct {
	Joke string `json:"joke"`
}

type Jokes struct {
	ID   uint
	DAY  string
	JOKE string
}
