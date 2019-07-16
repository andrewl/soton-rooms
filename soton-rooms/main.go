package main

import (
	"bytes"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/knakk/rdf"
	"net/http"
	"strings"
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse

type SlackAttachment struct {
	image_url string
}

type SlackResponse struct {
	text        string
	attachments [1]SlackAttachment
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(req events.APIGatewayProxyRequest) (Response, error) {

	var sentMessage = ""
	for _, field := range strings.Split(req.Body, "&") {
		f := strings.Split(field, "=")
		if f[0] == "text" {
			sentMessage = f[1]
			break
		}
	}

	if sentMessage == "" {
		//@todo: handle error
	}

	var buf bytes.Buffer
	var message string
	var url = "https://data.southampton.ac.uk/room/" + sentMessage + ".ttl"
	allFeatures := make(map[string]string)

	ttl_resp, err := http.Get(url)
	if err != nil {
		//@todo: handle error
	} else {
		defer ttl_resp.Body.Close()

		decoder := rdf.NewTripleDecoder(ttl_resp.Body, rdf.Turtle)
		allTriples, err := decoder.DecodeAll()

		if err != nil {
			//@todo: handle error
		} else {

			//get the room features and anything else we want to report on
			for _, triple := range allTriples {
				if triple.Obj.String() == "http://id.southampton.ac.uk/ns/RoomFeatureClass" {
					allFeatures[triple.Subj.String()] = ""
				}
				if triple.Pred.String() == "http://purl.org/openorg/capacity" {
					allFeatures["capacity"] = triple.Obj.String()
				}
				if triple.Pred.String() == "http://xmlns.com/foaf/0.1/depiction" {
					allFeatures["depiction"] = triple.Subj.String()
				}
				if triple.Pred.String() == "http://www.w3.org/2000/01/rdf-schema#label" && triple.Subj.String() == "http://id.southampton.ac.uk/room/"+sentMessage {
					name := ""
					ok := false
					if name, ok = allFeatures["name"]; ok {
						name = name + ", " + triple.Obj.String()
					} else {
						name = triple.Obj.String()
					}
					allFeatures["name"] = name
				}

			}

			//get the labels related to the feature subjects
			for _, triple := range allTriples {
				if triple.Pred.String() == "http://www.w3.org/2000/01/rdf-schema#label" {
					if _, ok := allFeatures[triple.Subj.String()]; ok {
						allFeatures[triple.Subj.String()] = triple.Obj.String()
					}
				}
			}

		}
	}

	if name, ok := allFeatures["name"]; ok {
		message = "Room: " + name + "\n"
		delete(allFeatures, "name")
	}

	if capacity, ok := allFeatures["capacity"]; ok {
		message = message + "Capacity: " + capacity + "\n"
		delete(allFeatures, "capacity")
	}

	allFeaturesArray := []string{}
	for _, v := range allFeatures {
		allFeaturesArray = append(allFeaturesArray, v)
	}
	message = message + "Room features: " + strings.Join(allFeaturesArray, ",") + "\n"
	//message = message + req.Body
	//@todo - fix attachments
	var slackAttachment [1]SlackAttachment

	if image_url, ok := allFeatures["depiction"]; ok {
		slackAttachment[0] = SlackAttachment{image_url}
		delete(allFeatures, "depiction")
	}

	slackResponse := SlackResponse{message, slackAttachment}
	payload, err := json.Marshal(slackResponse)

	payload, err = json.Marshal(map[string]interface{}{
		"text": message,
	})

	if err != nil {
		return Response{StatusCode: 404}, err
	}
	json.HTMLEscape(&buf, payload)

	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers: map[string]string{
			"Content-Type":          "application/json",
			"X-shining-web-handler": "soton-rooms-handler",
		},
	}

	return resp, nil
}

func main() {
	lambda.Start(Handler)
}
