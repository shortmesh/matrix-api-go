package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	_ "sherlock/matrix/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
	// "maunium.net/go/mautrix/id"
)

// @title           ShortMesh API
// @version         1.0
// @description     ShortMesh is a Matrix-based messaging bridge API that enables seamless communication across different messaging platforms.
// @description     It provides endpoints for user management, message sending, and platform bridging capabilities.
// @description     The API supports E.164 phone number format for contacts and implements secure authentication mechanisms.
// @description     The API supports the following platforms:
// @description     - WhatsApp
// @description     - Signal (coming soon)
// @host      localhost:8080
// @BasePath  /
// @schemes   http https

// Users represents a user entity
// @Description Represents a user structure with a name
// @name Users
// @type object
type Users struct {
	Username    string `json:"username"`
	ID          int    `json:"id"`
	AccessToken string `json:"access_token"`
}

// ClientJsonRequest represents login or registration data
// @Description Request payload for user login or registration
// @name ClientJsonRequest
// @type object
type ClientJsonRequest struct {
	Username string `json:"username" example:"john_doe"`
	Password string `json:"password" example:"securepassword123"`
}

// ClientMessageJsonRequeset represents a message sending request
// @Description Request payload to send a message to a room
// @name ClientMessageJsonRequeset
// @type object
type ClientMessageJsonRequeset struct {
	Username    string `json:"username" example:"john_doe"`
	AccessToken string `json:"access_token" example:"syt_YWxwaGE..."`
	Message     string `json:"message" example:"Hello, world!"`
}

// ClientBridgeJsonRequest represents bridge connection details
// @Description Request payload to bind a platform bridge to a user
// @name ClientBridgeJsonRequest
// @type object
type ClientBridgeJsonRequest struct {
	Username    string `json:"username" example:"john_doe"`
	AccessToken string `json:"access_token" example:"syt_YWxwaGE..."`
}

// LoginResponse represents the response for successful login
// @Description Response payload for successful login
type LoginResponse struct {
	Username    string `json:"username" example:"john_doe"`
	AccessToken string `json:"access_token" example:"syt_YWxwaGE..."`
	Status      string `json:"status" example:"logged in"`
}

// ErrorResponse represents an error response
// @Description Response payload for error cases
type ErrorResponse struct {
	Error   string `json:"error" example:"Invalid request"`
	Details string `json:"details,omitempty" example:"Username must be 3-32 characters"`
}

// MessageResponse represents the response for successful message sending
// @Description Response payload for successful message sending
type MessageResponse struct {
	Contact string `json:"contact" example:"+1234567890"`
	EventID string `json:"event_id" example:"$1234567890abcdef"`
	Message string `json:"message" example:"Hello, world!"`
	Status  string `json:"status" example:"sent"`
}

// DeviceResponse represents the response for successful device addition
// @Description Response payload for successful device addition. The websocket_url is used to establish a connection that:
// @Description - Receives media/images from the platform bridge
// @Description - Handles login synchronization events
// @Description - Receives existing active sessions if available
// @Description - Closes when receiving nil data (indicating end of session or error)
type DeviceResponse struct {
	WebsocketURL string `json:"websocket_url" example:"ws://localhost:8080/ws/telegram/john_doe"`
}

// Input validation functions
func sanitizeUsername(username string) (string, error) {
	// Remove any whitespace
	username = strings.TrimSpace(username)

	// Username should be 3-32 characters and contain only letters, numbers, and underscores
	validUsername := regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
	if !validUsername.MatchString(username) {
		return "", fmt.Errorf("username must be 3-32 characters and contain only letters, numbers, and underscores")
	}

	return username, nil
}

func sanitizePassword(password string) (string, error) {
	// Remove any whitespace
	password = strings.TrimSpace(password)

	// Password should be at least 7 characters
	if len(password) < 7 {
		return "", fmt.Errorf("password must be at least 7 characters long")
	}

	return password, nil
}

func sanitizeMessage(message string) (string, error) {
	// Remove any whitespace
	message = strings.TrimSpace(message)

	// Message should not be empty and have a reasonable length
	if len(message) == 0 {
		return "", fmt.Errorf("message cannot be empty")
	}
	if len(message) > 4096 {
		return "", fmt.Errorf("message is too long (max 4096 characters)")
	}

	return message, nil
}

func sanitizePlatform(platform string) (string, error) {
	// Remove any whitespace and convert to lowercase
	platform = strings.ToLower(strings.TrimSpace(platform))

	// Platform should be 2-20 characters and contain only letters and numbers
	validPlatform := regexp.MustCompile(`^[a-z0-9]{2,20}$`)
	if !validPlatform.MatchString(platform) {
		return "", fmt.Errorf("platform name must be 2-20 characters and contain only letters and numbers")
	}

	return platform, nil
}

func sanitizeContact(contact string) (string, error) {
	// Remove any whitespace
	contact = strings.TrimSpace(contact)

	// Remove plus sign if present
	contact = strings.TrimPrefix(contact, "+")

	// E.164 format validation: [country code][number], total length 8-15 digits
	validContact := regexp.MustCompile(`^[1-9]\d{7,14}$`)
	if !validContact.MatchString(contact) {
		return "", fmt.Errorf("contact must be a valid E.164 phone number (e.g., 1234567890 or +1234567890)")
	}

	return contact, nil
}

// ApiLogin godoc
// @Summary Logs a user into the Matrix server
// @Description Authenticates a user and returns an access token
// @Accept  json
// @Produce  json
// @Param   payload body ClientJsonRequest true "Login Credentials"
// @Success 200 {object} LoginResponse "Successfully logged in"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Login failed"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /login [post]
func ApiLogin(c *gin.Context) {
	var clientJsonRequest ClientJsonRequest

	if err := c.BindJSON(&clientJsonRequest); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Sanitize inputs
	username, err := sanitizeUsername(clientJsonRequest.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	password, err := sanitizePassword(clientJsonRequest.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	homeServer := cfg.HomeServer

	client, err := mautrix.NewClient(homeServer, id.NewUserID(username, cfg.HomeServerDomain), cfg.User.AccessToken)
	if err != nil {
		log.Printf("Failed to create Matrix client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	controller := Controller{
		Client:   client,
		Username: username,
		UserID:   client.UserID,
	}
	if err := controller.LoginProcess(password); err != nil {
		log.Printf("Login failed for %s: %v", username, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Login failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":     username,
		"access_token": client.AccessToken,
		"status":       "logged in",
	})
}

// ApiCreate godoc
// @Summary Creates a new user on the Matrix server
// @Description Registers a new user and returns an access token
// @Accept  json
// @Produce  json
// @Param   payload body ClientJsonRequest true "User Registration"
// @Success 201 {object} LoginResponse "Successfully created user"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 409 {object} ErrorResponse "User creation failed"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router / [post]
func ApiCreate(c *gin.Context) {
	var clientJsonRequest ClientJsonRequest

	if err := c.BindJSON(&clientJsonRequest); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Sanitize inputs
	username, err := sanitizeUsername(clientJsonRequest.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	password, err := sanitizePassword(clientJsonRequest.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	homeServer := cfg.HomeServer

	client, err := mautrix.NewClient(homeServer, "", "")

	if err != nil {
		log.Printf("Failed to create Matrix client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	controller := Controller{
		Client:   client,
		Username: username,
	}

	if err := controller.CreateProcess(password); err != nil {
		log.Printf("User creation failed for %s: %v\n", username, err)
		c.JSON(http.StatusConflict, gin.H{"error": "User creation failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"username":     username,
		"access_token": client.AccessToken,
		"status":       "created",
	})
}

// ApiSendMessage godoc
// @Summary Sends a message to a specified room
// @Description Sends a message to a contact through the specified platform bridge
// @Accept  json
// @Produce  json
// @Param   platform path string true "Platform Name" example:"telegram"
// @Param   contact path string true "Contact ID (E.164 phone number without the plus)" example:"1234567890"
// @Param   payload body ClientMessageJsonRequeset true "Message Payload"
// @Success 200 {object} MessageResponse "Message sent successfully"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Failed to send message"
// @Router /{platform}/message/{contact} [post]
func ApiSendMessage(c *gin.Context) {
	var req ClientMessageJsonRequeset

	// Sanitize platform and contact parameters
	_, err := sanitizePlatform(c.Param("platform"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contactID, err := sanitizeContact(c.Param("contact"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.BindJSON(&req); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Validate required fields
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}
	if req.AccessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access token is required"})
		return
	}
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message is required"})
		return
	}

	// Sanitize username
	username, err := sanitizeUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Sanitize message
	message, err := sanitizeMessage(req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg, _ := (&Conf{}).getConf()
	homeServer := cfg.HomeServer

	client, err := mautrix.NewClient(homeServer, id.NewUserID(username, cfg.HomeServerDomain), req.AccessToken)
	if err != nil {
		log.Printf("Failed to create Matrix client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not initialize client"})
		return
	}

	// Validate access token
	matrixClient := MatrixClient{
		Client: client,
	}
	_, err = matrixClient.LoadActiveSessionsByAccessToken(req.AccessToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token", "details": err.Error()})
		return
	}

	controller := Controller{
		Client: client,
		UserID: client.UserID,
	}

	err = controller.SendMessage(username, message, contactID, c.Param("platform"))
	if err != nil {
		log.Printf("Failed to send message: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contact": contactID,
		"message": message,
		"status":  "sent",
	})
}

// ApiAddDevice godoc
// @Summary Adds a device for a given platform
// @Description Registers a new device connection for the specified platform and establishes a websocket connection.
// @Description The websocket connection will:
// @Description - Receive media/images from the platform bridge
// @Description - Handle login synchronization events
// @Description - Send existing active sessions if available
// @Description - Close connection when receiving nil data (indicating end of session or error)
// @Description Here are various platforms supported:
// @Description 'wa' (for WhatsApp)
// @Description 'signal' (for Signal)
// @Accept  json
// @Produce  json
// @Param   platform path string true "Platform Name" example:"wa"
// @Param   payload body ClientBridgeJsonRequest true "Device Payload"
// @Success 200 {object} DeviceResponse "Successfully added device and established websocket connection"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Invalid access token"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /{platform}/devices [post]
func ApiAddDevice(c *gin.Context) {
	var bridgeJsonRequest ClientBridgeJsonRequest

	// Sanitize platform parameter
	platformName, err := sanitizePlatform(c.Param("platform"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.ShouldBindJSON(&bridgeJsonRequest); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Sanitize username
	username, err := sanitizeUsername(bridgeJsonRequest.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := mautrix.NewClient(
		cfg.HomeServer, id.NewUserID(username, cfg.HomeServerDomain), bridgeJsonRequest.AccessToken)

	if err != nil {
		log.Printf("Failed to create Matrix client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not initialize client"})
		return
	}

	matrixClient := MatrixClient{
		Client: client,
	}
	_, err = matrixClient.LoadActiveSessionsByAccessToken(bridgeJsonRequest.AccessToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token", "details": err.Error()})
		return
	}

	bridge := &Bridges{}

	for _, _bridge := range syncingClients.Users[username].MsgBridges {
		if _bridge.Name == platformName {
			bridge = _bridge
		}
	}

	if bridge.Name == "" {
		bridge.Name = platformName
		bridge.ChLoginSyncEvt = make(chan *event.Event, 500)
		bridge.ChImageSyncEvt = make(chan []byte, 500)
		bridge.ChMsgEvt = make(chan *event.Event, 500)
		bridge.Client = client
	}
	log.Println("Adding bridge:", bridge)

	syncingClients.Users[username].MsgBridges = append(syncingClients.Users[username].MsgBridges, bridge)

	ws := Websockets{Bridge: bridge}

	websocketUrl := ""
	if index := GetWebsocketIndex(username, platformName); index > -1 {
		websocketUrl = GlobalWebsocketConnection.Registry[index].Url
	} else {
		websocketUrl = ws.RegisterWebsocket(platformName, username)
	}

	c.JSON(http.StatusOK, gin.H{
		"websocket_url": string(websocketUrl),
	})
}

func main() {
	if cfgError != nil {
		panic(cfgError)
	}

	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	router.POST("/", ApiCreate)
	router.POST("/login", ApiLogin)
	router.POST("/:platform/message/:contact", ApiSendMessage)
	router.POST("/:platform/devices", ApiAddDevice)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	ks.Init()

	host := cfg.Server.Host
	port := cfg.Server.Port

	tlsCert := cfg.Server.Tls.Crt
	tlsKey := cfg.Server.Tls.Key

	go func() {
		err := (&MatrixClient{}).SyncAllClients()
		if err != nil {
			panic(err)
		}
	}()

	if cfg.Websocket.Tls.Crt != "" && cfg.Websocket.Tls.Key != "" {
		go func() {
			err := MainWebsocket(true)
			if err != nil {
				panic(err)
			}
		}()
	} else {
		go func() {
			err := MainWebsocket(false)
			if err != nil {
				panic(err)
			}
		}()
	}

	if tlsCert != "" && tlsKey != "" {
		router.RunTLS(fmt.Sprintf(":%s", port), tlsCert, tlsKey)
	} else {
		router.Run(fmt.Sprintf("%s:%s", host, port))
	}
}
