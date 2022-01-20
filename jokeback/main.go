package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/rs/zerolog/log"
	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
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

	for name, values := range r.Header {
		// Loop over all values for the name.
		for _, value := range values {
			log.Debug().Str(name, value).Msg("Headers")
		}
	}

	ctx, err := tracer.Extract(tracer.HTTPHeadersCarrier(r.Header))
	if err != nil {
		log.Debug().Str("dd-tracer-go", "unable to extract parent trace id").Msg(err.Error())
	}
	span := tracer.StartSpan("process.todaysJoke", tracer.ChildOf(ctx))

	defer span.Finish()
	log.Trace().Uint64("trace_id", span.Context().SpanID()).Msg("tracking...")

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
	switch msg.Weekday {
	case "Sunday":
		todaysJokeContent = "What do you call a boomerang that won't cone back? - A stick."
	case "Monday":
		todaysJokeContent = "What does a cloud wear under his raincoat? -Thunderwear."
	case "Tuesday":
		todaysJokeContent = "Two pickles fell out of a jar onto the floor. What did one say to the other? -Dill with it."
	case "Wednesday":
		todaysJokeContent = "What time is it when the clock strikes 13? -Time to get a new clock."
	case "Thursday":
		todaysJokeContent = "How does a cucumber become a pickle? -It goes through a jarring experience."
	case "Friday":
		todaysJokeContent = "What did one toilet say to the other? -You look a bit flushed."
	case "Saturday":
		todaysJokeContent = "Why did the dinosaur cross the road? -Because the chicken wasn’t born yet."
	default:
		todaysJokeContent = "It's not a weekday today."
	}

	todaysJoke := joke{Joke: todaysJokeContent}
	req, err := json.Marshal(todaysJoke)
	if err != nil {
		log.Panic().Msg(err.Error())
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(req))
}

type message struct {
	Weekday string `json:"weekday"`
}

type joke struct {
	Joke string `json:"joke"`
}
