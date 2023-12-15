package masa

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func NewBridge() error {

	logrus.Info("starting server")
	router := gin.Default()
	//router.Use(cors.Default())
	// router.SetTrustedProxies([]string{add values here})

	// Use the auth middleware for the /webhook route
	router.POST("/webhook", authMiddleware(), webhookHandler)

	// Paths to the certificate and key files
	certFile := os.Getenv(Cert)
	keyFile := os.Getenv(CertPem)

	if err := router.RunTLS(":8080", certFile, keyFile); err != nil {
		return err
	}
	return nil
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the token from the Authorization header
		token := c.GetHeader("Authorization")

		// Check the token
		if token != "your_expected_token" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}

func webhookHandler(c *gin.Context) {
	// Handle the webhook request here

	c.JSON(http.StatusOK, gin.H{"message": "Webhook called"})
}
