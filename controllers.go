package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

var cfg, cfgError = (&Conf{}).getConf()
var GlobalWebsocketConnection = WebsocketData{
	ch:       make(chan []byte, 500),
	Registry: make([]*WebsocketMap, 0),
}

var ks = Keystore{
	filepath: cfg.KeystoreFilepath,
}

var (
	syncingClients = SyncingClients{
		Bridge:   make(map[string][]*Bridges),
		Registry: make(map[string]bool),
	}
	mapMutex = sync.Mutex{}
)

func JoinRooms(
	client *mautrix.Client,
	bridge *Bridges,
	username string,
) error {
	clientDB := ClientDB{
		username: username,
		filepath: "db/" + username + ".db",
	}
	clientDB.Init()

	clientRooms, err := bridge.Room.JoinedRooms(client)
	if err != nil {
		return err
	}
	log.Println("Client Rooms: ", clientRooms)

	for _, entry := range cfg.Bridges {
		for name, config := range entry {
			if fRoooms, err := clientDB.FetchRoomsByMembers(name); fRoooms.ID == "" && err == nil {
				log.Println("[+] Creating room for:", name, config.BotName)
				bridge.Room.User.Username = username
				roomId, err := bridge.Room.CreateRoom(
					client, name, config.BotName, RoomTypeManagement, true,
				)
				if err != nil {
					return err
				}
				log.Println("[+] Created room successfully for:", config.BotName, roomId)
			}
		}
	}
	return nil
}

func CreateProcess(
	client *mautrix.Client,
	bridge *Bridges,
	username string,
	password string,
) error {
	_, err := Create(client, username, password, bridge)

	if err != nil {
		return err
	}

	log.Println("[+] Created user: ", username)

	client.UserID = id.NewUserID(username, cfg.HomeServerDomain)

	return nil
}

func LoginProcess(
	client *mautrix.Client,
	bridge *Bridges,
	username string,
	password string,
) error {
	_, err := LoadActiveSessions(client, username, password, bridge)
	if err != nil {
		if _, err = Login(client, username, password, bridge); err != nil {
			return err
		}
	}

	return nil
}

func CompleteRun(
	client *mautrix.Client,
	bridge *Bridges,
) {
	if len(client.AccessToken) < 3 {
		log.Fatalf("Client access token expected: > 2, got: %d %v", len(client.AccessToken), client.AccessToken)
		return
	}

	callback := func(inMd IncomingMessageMetaData, err error) {
		if err != nil {
			log.Println(err)
			return
		}
		switch inMd.Message.Type {
		case "m.text":
			log.Printf(">> %s %v\n", inMd.Type, inMd)
		case "m.image":
			rawImage, err := ParseImage(client, string(inMd.Message.Content.AsMessage().URL))
			if err != nil {
				panic(err)
			}

			filename := inMd.Message.Content.AsMessage().FileName
			if filename == "" {
				filename = inMd.Message.Content.AsMessage().Body
			}
			imageDownloadFilepath := "downloads/rooms/" + filename
			os.WriteFile(imageDownloadFilepath, rawImage, 0644)
			log.Printf("[+] Saved image to room dir: %s\n", imageDownloadFilepath)
		default:
			log.Printf("[-] Type not yet implemented: %v\n", inMd.Message.Content.Raw["msgtype"])
		}
	}

	go func() {
		bridge.Room.ListenJoinedRooms(client, callback)
	}()

	go func() {
		err := Sync(client, bridge)
		if err != nil {
			panic(err)
		}
	}()

	messagingRoomId := ""
	reader := bufio.NewReader(os.Stdin)

	go func() {
		for {
			fmt.Printf("[%s]-> ", messagingRoomId)
			text, _ := reader.ReadString('\n')

			if text == "" || text == "\n" {
				continue
			}

			text = strings.TrimSuffix(text, "\n")

			if strings.Contains(text, ">room") {
				st := strings.Split(text, " ")
				messagingRoomId = st[len(st)-1]
				continue
			}

			if messagingRoomId == "" {
				fmt.Println("** Messaging requires a room")
				continue
			}

			room := Rooms{
				ID: id.RoomID(messagingRoomId),
			}

			resp, err := room.SendRoomMessages(client, text)
			if err != nil {
				fmt.Printf("%v\n", err)
			}

			fmt.Println(resp)
		}

	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	client.StopSync()
	fmt.Println("\nShutdown signal received. Exiting...")

	os.Exit(0)
}
