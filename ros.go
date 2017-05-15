package goros

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	//"golang.org/x/net/websocket"
	"github.com/gorilla/websocket"
)

var (
	messageCount = 0
)

type Base struct {
	Op string `json:"op"`
	Id string `json:"id"`
}

type Ros struct {
	//origin           string
	url              string
	ws               *websocket.Conn
	receivedMapMutex sync.Mutex
	receivedMap      map[string]chan interface{}
	IsConnected      bool // exported
	IsSubscribed     bool // exported
}

func NewRos(url string) (*Ros, error) {
	//ros := Ros{url: url, origin: "https://localhost"}
	ros := Ros{url: url}
	ros.receivedMap = make(map[string]chan interface{})
	err := ros.connect()
	if err != nil {
		return nil, fmt.Errorf("goros.NewRos: %v", err)
	}
	go ros.handleIncoming()
	return &ros, nil
}

func (ros *Ros) connect() error {
	//ws, err := websocket.Dial(ros.url, "", ros.origin)
	dialer := websocket.Dialer{}
	ws, _, err := dialer.Dial(ros.url, nil)
	if err != nil {
		//log.Fatal(err)
		return fmt.Errorf("goros.connect: %v", err)
	}
	ros.IsConnected = true
	ros.ws = ws
	return nil
}

func (ros *Ros) getServiceResponse(service *ServiceCall) *ServiceResponse {
	response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[service.Id] = response
	ros.receivedMapMutex.Unlock()
	//err := websocket.JSON.Send(ros.ws, service)
	err := ros.ws.WriteJSON(service)
	if err != nil {
		fmt.Println("Couldn't send msg")
	}

	serviceResponse := <-response
	return serviceResponse.(*ServiceResponse)
}

func (ros *Ros) getTopicResponse(topic *Topic) *interface{} {
	response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[topic.Id] = response
	ros.receivedMapMutex.Unlock()
	//err := websocket.JSON.Send(ros.ws, topic)
	err := ros.ws.WriteJSON(topic)
	if err != nil {
		fmt.Println("Couldn't send msg")
	}
	log.Println(ros.receivedMap)

	topicResponse := <-response
	return &topicResponse
}

func (ros *Ros) returnToAppropriateChannel(id string, data interface{}) {
	ros.receivedMapMutex.Lock()
	ros.receivedMap[id] <- data
	ros.receivedMapMutex.Unlock()
}

func (ros *Ros) handleIncoming() {
	//var msg []byte
	//var err error
	for {
		//err := websocket.Message.Receive(ros.ws, &msg)
		//_, msg, err = ros.ws.ReadMessage()
		_, msg, err := ros.ws.ReadMessage()
		if err != nil {
			//log.Printf("DBG: goros.handleIncoming: ros before: %v" , ros)
			//ros.ws = nil
			ros.IsConnected = false
			//ros.IsSubscribed = false
			if err == io.EOF {
				break
			}
			fmt.Println("goros.handleIncoming: Couldn't receive msg " + err.Error())
			//log.Printf("DBG: goros.handleIncoming: ros after : %v" , ros)
			break
		}

		/*
			opRegex, err := regexp.Compile(`"op"\s*:\s*"[[:alpha:],_]*`)
			if err != nil {
				log.Println(err)
			}
			opString := opRegex.FindString(string(msg))
			splitOpString := strings.Split(opString, "\"")
			operation := splitOpString[len(splitOpString)-1]
		*/

		var base Base
		json.Unmarshal(msg, &base)

		log.Println(base)

		if base.Op == "service_response" {
			var serviceResponse ServiceResponse
			json.Unmarshal(msg, &serviceResponse)
			ros.receivedMapMutex.Lock()
			ros.receivedMap[serviceResponse.Id] <- &serviceResponse
			ros.receivedMapMutex.Unlock()
		} else if base.Op == "publish" {
			log.Println(base)
			var topic Topic
			json.Unmarshal(msg, &topic)
			ros.receivedMapMutex.Lock()
			ros.receivedMap[topic.Topic] <- &topic
			ros.receivedMapMutex.Unlock()
		}
	}
}

func (ros *Ros) GetTopics() []string {
	response := ros.getServiceResponse(newServiceCall("/rosapi/topics"))
	var topics []string
	json.Unmarshal(response.Values["topics"], &topics)
	return topics
}

func (ros *Ros) GetServices() []string {
	response := ros.getServiceResponse(newServiceCall("/rosapi/services"))
	var services []string
	json.Unmarshal(response.Values["services"], &services)
	return services
}

func (ros *Ros) GetParams() []string {
        response := ros.getServiceResponse(newServiceCall("/rosapi/get_param_names"))
        var params []string
        json.Unmarshal(response.Values["names"], &params)
        return params
}

func (ros *Ros) Subscribe(topicName string, callback TopicCallback) {
	//topicResponse := ros.getTopicResponse(topic)
	topic := NewTopic(topicName)
	err := ros.SubscribeTopic(topic, callback)
	if err != nil {
		fmt.Println("Couldn't send msg")
	}
}

func (ros *Ros) SubscribeTopic(topic *Topic, callback TopicCallback) error {
	response := make(chan interface{})
	err := ros.SubscribeTopicWithChannel(topic, &response)

	if err != nil {
		return fmt.Errorf("goros.SubscribeTopic: %v", err)
	}
	go func() {
		for {
			callback(&(<-response).(*Topic).Msg)
		}
	}()
	return nil
}

func (ros *Ros) SubscribeTopicWithChannel(topic *Topic, response *chan interface{}) error {
	topic.Op = "subscribe"
	tmptopics := ros.GetTopics()
	ok := false
	for _, tmptopic := range tmptopics {
		if topic.Topic == tmptopic {
			ok = true
			break
		}
	}
	if ok == false {
		return fmt.Errorf("goros.SubscribeTopicWithChannel: Could not find topic: %s", topic.Topic)
	}
	SetNewTopicId(topic)
	log.Printf("DBG: goros.SubscribeTopicWithChannel: topic : %v" , topic)
	log.Printf("DBG: goros.SubscribeTopicWithChannel: ros   : %v" , ros)
	//response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[topic.Topic] = *response
	ros.receivedMapMutex.Unlock()
	//err := websocket.JSON.Send(ros.ws, *topic)
	err := ros.ws.WriteJSON(topic)
	if err != nil {
		//fmt.Println("Couldn't send msg")
		return fmt.Errorf("goros.SubscribeTopicWithChannel: %v", err)
	}
	ros.IsSubscribed = true
	return nil
}

func (ros *Ros) OutboundTopic(topic *Topic) error {
	SetNewTopicId(topic)
	log.Printf("DBG: goros.OutboundTopic: topic : %v" , topic)
	log.Printf("DBG: goros.OutboundTopic: ros   : %v" , ros)
	//err := websocket.JSON.Send(ros.ws, *topic)
	err := ros.ws.WriteJSON(topic)
	if err != nil {
		return fmt.Errorf("goros.OutboundTopic: %v", err)
	}
	return nil
}

func (ros *Ros) AdvertiseTopic(topic *Topic) error {
	topic.Op = "advertise"
	err := ros.OutboundTopic(topic)
	if err != nil {
		return fmt.Errorf("goros.AdvertiseTopic: %v", err)
	}
	return nil
}

func (ros *Ros) PublishTopic(topic *Topic) error {
	topic.Op = "publish"
	err := ros.OutboundTopic(topic)
	if err != nil {
		return fmt.Errorf("goros.PublishTopic: %v", err)
	}
	return nil
}


