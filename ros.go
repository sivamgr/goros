package goros

import (
	"encoding/json"
	"fmt"
	//"io"
	"log"
	"sync"

	//"golang.org/x/net/websocket"
	"github.com/gorilla/websocket"
	"time"
)

const (
	TimeoutInSec = 5
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
	Url              string
	Ws               *websocket.Conn //exported
	receivedMapMutex sync.Mutex
	receivedMap      map[string]chan interface{}
	WsWriteMutex     sync.Mutex
}

type ArgGetParam struct {
	Name    string `json:"name"`
	Default string `json:"default,omitempty"`
}

type ArgSetParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ArgPublishers struct {
	Topic string `json:"topic"`
}

func NewRos(url string) (*Ros, error) {
	ros := Ros{Url: url}
	ros.receivedMap = make(map[string]chan interface{})
	err := ros.connect()
	if err != nil {
		return nil, fmt.Errorf("goros.NewRos: %v", err)
	}
	go ros.handleIncoming()
	return &ros, nil
}

func (ros *Ros) connect() error {
	dialer := websocket.Dialer{}
	ws, _, err := dialer.Dial(ros.Url, nil)
	if err != nil {
		return fmt.Errorf("goros.connect: %v", err)
	}
	ros.Ws = ws
	return nil
}

func (ros *Ros) getServiceResponse(service *ServiceCall) (*ServiceResponse, error) {
	response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[service.Id] = response
	ros.receivedMapMutex.Unlock()
	var serviceResponse interface{}
	ros.WsWriteMutex.Lock()
	err := ros.Ws.WriteJSON(service)
	ros.WsWriteMutex.Unlock()
	if err != nil {
		//fmt.Println("Couldn't send msg")
		return nil, fmt.Errorf("goros.getServiceResponse: Couldn't send msg: %v", err)
	}

	select {
	case serviceResponse = <-response:
		break
	case <-time.After(TimeoutInSec * time.Second):
		return nil, fmt.Errorf("goros.getServiceResponse: Timeout %d sec., no response.", TimeoutInSec)
	}
	return serviceResponse.(*ServiceResponse), err
}

func (ros *Ros) getTopicResponse(topic *Topic) *interface{} {
	response := make(chan interface{})
	ros.receivedMapMutex.Lock()
	ros.receivedMap[topic.Id] = response
	ros.receivedMapMutex.Unlock()
	ros.WsWriteMutex.Lock()
	err := ros.Ws.WriteJSON(topic)
	ros.WsWriteMutex.Unlock()
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
	for {
		_, msg, err := ros.Ws.ReadMessage()
		if err != nil {
			//log.Printf("DBG: goros.handleIncoming: ros before: %v" , ros)
			//log.Printf("DBG: goros.handleIncoming: err: %v" , err)
			//err := ros.Ws.Close()
			//log.Printf("DBG: goros.handleIncoming: disconnect and close: ros.Ws: %v, err: %v", ros.Ws , err)
			ros.Ws = nil
			/*
			if err == io.EOF {
				log.Println("goros.handleIncoming: Couldn't receive msg.  " + err.Error())
				break
			}
			*/
			//log.Println("goros.handleIncoming: Couldn't receive msg. Socket disconnected. " + err.Error())
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

		//log.Println(base)
		//log.Printf("goros.handleIncoming: Info: %v",  base)

		if base.Op == "service_response" {
			var serviceResponse ServiceResponse
			json.Unmarshal(msg, &serviceResponse)
			ros.receivedMapMutex.Lock()
			ros.receivedMap[serviceResponse.Id] <- &serviceResponse
			ros.receivedMapMutex.Unlock()
		} else if base.Op == "publish" {
			//log.Printf("goros.handleIncoming: %v",  base)
			var topic Topic
			json.Unmarshal(msg, &topic)
			ros.receivedMapMutex.Lock()
			ros.receivedMap[topic.Topic] <- &topic
			ros.receivedMapMutex.Unlock()
		}
	}
}

func (ros *Ros) GetTopics() ([]string, error) {
	response, err := ros.getServiceResponse(newServiceCall("/rosapi/topics"))
	if err != nil {
		err = fmt.Errorf("goros.GetTopics: %v", err)
	}
	var topics []string
	json.Unmarshal(response.Values["topics"], &topics)
	return topics, err
}

func (ros *Ros) GetPublishers(topicName string) ([]string, error) {
        var publishers []string
	arg, err := json.Marshal(ArgPublishers{Topic: topicName})
	if err != nil {
		return publishers, fmt.Errorf("goros.GetPublishers: %v", err)
	}
	var response *ServiceResponse
	serviceCall := newServiceCall("/rosapi/publishers")
	serviceCall.Args = arg
	//serviceCall.Args = (json.RawMessage)(`{"name":"` + topicName + `"}`)
        response, err = ros.getServiceResponse(serviceCall)
	if err != nil {
		return publishers, fmt.Errorf("goros.GetPublishers: %v", err)
	}
	//log.Printf("DBG: goros.GetPublishers: response v : %v" , *response)
	//log.Printf("DBG: goros.GetPublishers: response s : %s" , *response)
	if response.Result == false {
		return publishers, fmt.Errorf("goros.GetParam: response result is false.")
	}
        json.Unmarshal(response.Values["publishers"], &publishers)
	//log.Printf("DBG: goros.GetPublishers: param : %s" , publishers)
        return publishers, err
}

func (ros *Ros) GetServices() ([]string , error){
	response, err := ros.getServiceResponse(newServiceCall("/rosapi/services"))
	if err != nil {
		err = fmt.Errorf("goros.GetServices: %v", err)
	}
	var services []string
	json.Unmarshal(response.Values["services"], &services)
	return services, err
}

func (ros *Ros) GetParams() ([]string, error) {
        response, err := ros.getServiceResponse(newServiceCall("/rosapi/get_param_names"))
	if err != nil {
		err = fmt.Errorf("goros.GetParams: %v", err)
	}
        var params []string
        json.Unmarshal(response.Values["names"], &params)
        return params, err
}

func (ros *Ros) GetParam(paramName string) (string, error) {
	arg, err := json.Marshal(ArgGetParam{Name: paramName})
	if err != nil {
		return "", fmt.Errorf("goros.GetParam: %v", err)
	}
	var response *ServiceResponse
	serviceCall := newServiceCall("/rosapi/get_param")
	serviceCall.Args = arg
	//serviceCall.Args = (json.RawMessage)(`{"name":"` + paramName + `"}`)
        response, err = ros.getServiceResponse(serviceCall)
	if err != nil {
		return "", fmt.Errorf("goros.GetParam: %v", err)
	}
	//log.Printf("DBG: goros.GetParam: response v : %v" , *response)
	//log.Printf("DBG: goros.GetParam: response s : %s" , *response)
        var param string
	if response.Result == false {
		return "", fmt.Errorf("goros.GetParam: response result is false.")
	}
        json.Unmarshal(response.Values["value"], &param)
	//log.Printf("DBG: goros.GetParam: param : %s" , param)
        return param, err
}

func (ros *Ros) SetParam(paramName string, value string) error {
	arg, err := json.Marshal(ArgSetParam{Name: paramName, Value: value})
	if err != nil {
		return fmt.Errorf("goros.SetParam: %v", err)
	}
	var response *ServiceResponse
	serviceCall := newServiceCall("/rosapi/set_param")
	serviceCall.Args = arg
	//serviceCall.Args = (json.RawMessage)(`{"name":"` + paramName + `"}`)
        response, err = ros.getServiceResponse(serviceCall)
	if err != nil {
		return fmt.Errorf("goros.SetParam: %v", err)
	}
	//log.Printf("DBG: goros.SetParam: response v : %v" , *response)
	//log.Printf("DBG: goros.SetParam: response s : %s" , *response)
	if response.Result == false {
		return fmt.Errorf("goros.SetParam: response result is false.")
	}
	//log.Printf("DBG: goros.SetParam: param : %s" , param)
        return err
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
	if topic.Throttle_rate < 0 {
		log.Printf("goros.SubscribeTopicWithChannel: Warn: throttle_rate(%d) is not allowed, Set to 0", topic.Throttle_rate)
		topic.Throttle_rate = 0
	}
	if topic.Queue_length < 1 {
		log.Printf("goros.SubscribeTopicWithChannel: Warn: queue_length(%d) is not allowed, Set to 1", topic.Queue_length)
		topic.Queue_length = 1
	}
	tmpPublishers, err := ros.GetPublishers(topic.Topic)
	if err != nil {
		return fmt.Errorf("goros.SubscribeTopicWithChannel: %v", err)
	}
	if len(tmpPublishers) == 0 {
		return fmt.Errorf("goros.SubscribeTopicWithChannel: Could not find topic: %s", topic.Topic)
	}
	SetNewTopicId(topic)
	//log.Printf("DBG: goros.SubscribeTopicWithChannel: topic : %v" , *topic)
	//log.Printf("DBG: goros.SubscribeTopicWithChannel: ros   : %v" , *ros)
	ros.receivedMapMutex.Lock()
	ros.receivedMap[topic.Topic] = *response
	ros.receivedMapMutex.Unlock()
	ros.WsWriteMutex.Lock()
	err = ros.Ws.WriteJSON(topic)
	ros.WsWriteMutex.Unlock()
	if err != nil {
		return fmt.Errorf("goros.SubscribeTopicWithChannel: %v", err)
	}
	return nil
}

func (ros *Ros) OutboundTopic(topic *Topic) error {
	SetNewTopicId(topic)
	//log.Printf("DBG: goros.OutboundTopic: topic : %v" , *topic)
	//log.Printf("DBG: goros.OutboundTopic: ros   : %v" , *ros)
	ros.WsWriteMutex.Lock()
	err := ros.Ws.WriteJSON(topic)
	ros.WsWriteMutex.Unlock()
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

func (ros *Ros) UnsubscribeTopic(topic *Topic) error {
	topic.Op = "unsubscribe"
	err := ros.OutboundTopic(topic)
	if err != nil {
		return fmt.Errorf("goros.UnsubscribeTopic: %v", err)
	}
	return nil
}


