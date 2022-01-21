package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func main() {
	tracer.Start(tracer.WithServiceName("jokefront"))
	defer tracer.Stop()

	r := muxtrace.NewRouter(muxtrace.WithServiceName("jokefront"))
	r.HandleFunc("/todaysjoke", todaysjokeHandler).Methods("GET")
	log.Print("Start listening on :8000...")
	err := http.ListenAndServe(":8000", r)
	if err != nil {
		log.Panic().Msg(err.Error())
	}
}

func todaysjokeHandler(w http.ResponseWriter, r *http.Request) {
	span, ctx := tracer.StartSpanFromContext(r.Context(), "func.todaysjokeHandler")
	defer span.Finish()

	msg := message{
		Weekday: time.Now().Weekday().String(),
	}
	log.Trace().Uint64("dd.trace_id", span.Context().TraceID()).
		Uint64("dd.span_id", span.Context().SpanID()).
		Str("dd.service", "jokefront").
		Str("dd.env", "prod").
		Str("dd.version", "1.0.0").
		Msg("today is " + msg.Weekday)

	content, err := json.Marshal(msg)
	if err != nil {
		log.Error().Uint64("dd.trace_id", span.Context().TraceID()).
			Uint64("dd.span_id", span.Context().SpanID()).
			Str("dd.service", "jokefront").
			Str("dd.env", "prod").
			Str("dd.version", "1.0.0").
			Msg(err.Error())
	}

	req, err := http.NewRequest("POST", "http://jokeback:7000/api/jokes", bytes.NewBuffer(content))
	if err != nil {
		log.Panic().Uint64("dd.trace_id", span.Context().TraceID()).
			Uint64("dd.span_id", span.Context().SpanID()).
			Str("dd.service", "jokefront").
			Str("dd.env", "prod").
			Str("dd.version", "1.0.0").
			Msg(err.Error())
	}

	client := http.Client{Timeout: time.Duration(2 * time.Second)}
	req = req.WithContext(ctx)
	err = tracer.Inject(span.Context(), tracer.HTTPHeadersCarrier(req.Header))
	if err != nil {
		log.Debug().Uint64("dd.trace_id", span.Context().TraceID()).
			Uint64("dd.span_id", span.Context().SpanID()).
			Str("dd.service", "jokefront").
			Str("dd.env", "prod").
			Str("dd.version", "1.0.0").
			Msg(err.Error())
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error().Uint64("dd.trace_id", span.Context().TraceID()).
			Uint64("dd.span_id", span.Context().SpanID()).
			Str("dd.service", "jokefront").
			Str("dd.env", "prod").
			Str("dd.version", "1.0.0").
			Msg(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "oops something wrong hapenned...")
		return
	}

	log.Trace().Msg("backend answered with " + resp.Status)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error().Uint64("dd.trace_id", span.Context().TraceID()).
			Uint64("dd.span_id", span.Context().SpanID()).
			Str("dd.service", "jokefront").
			Str("dd.env", "prod").
			Str("dd.version", "1.0.0").
			Msg(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "oops something wrong hapenned...")
		return
	}
	log.Trace().Uint64("dd.trace_id", span.Context().TraceID()).
		Uint64("dd.span_id", span.Context().SpanID()).
		Str("dd.service", "jokefront").
		Str("dd.env", "prod").
		Str("dd.version", "1.0.0").
		Msg("body: " + string(body))

	jokeOfTheDay := joke{}
	err = json.Unmarshal(body, &jokeOfTheDay)
	if err != nil {
		log.Error().Uint64("dd.trace_id", span.Context().TraceID()).
			Uint64("dd.span_id", span.Context().SpanID()).
			Str("dd.service", "jokefront").
			Str("dd.env", "prod").
			Str("dd.version", "1.0.0").
			Msg(err.Error())
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, jokeOfTheDay.Joke)
}

type message struct {
	Weekday string `json:"weekday"`
}

type joke struct {
	Joke string `json:"joke"`
}
