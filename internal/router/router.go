package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zht475706171/TaiSang-KB/internal/handler"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/crypto"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/storage"
	"github.com/zht475706171/TaiSang-KB/internal/service/chat"
	"github.com/zht475706171/TaiSang-KB/internal/service/chunker"
	"github.com/zht475706171/TaiSang-KB/internal/service/document"
	"github.com/zht475706171/TaiSang-KB/internal/service/kb"
	"github.com/zht475706171/TaiSang-KB/internal/service/setting"
	"github.com/zht475706171/TaiSang-KB/internal/worker"
	"gorm.io/gorm"
)

type Config struct {
	ListenAddr    string
	StorageDir    string
	EncryptionKey string
}

type Services struct {
	KB       *kb.Service
	Document *document.Service
	Chat     *chat.Service
	Setting  *setting.Service
	Worker   *worker.ParseWorker
}

func NewServices(db *gorm.DB, cfg Config) (*Services, error) {
	st := storage.New(cfg.StorageDir)
	c, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}

	settingRepo := postgres.NewSettingRepository(db)
	kbRepo := postgres.NewKBRepository(db)
	docRepo := postgres.NewDocumentRepository(db)
	chunkRepo := postgres.NewChunkRepository(db)
	chatRepo := postgres.NewChatRepository(db)

	settingSvc := setting.NewService(settingRepo, c)
	s, _ := settingSvc.GetDecrypted()
	var embedC *llmclient.EmbeddingClient
	var chatC *llmclient.ChatClient
	var rerankC *llmclient.RerankClient
	if s != nil && s.APIKey != "" {
		embedC = llmclient.NewEmbeddingClient(s.APIBaseURL, s.APIKey, s.EmbeddingModel, 64)
		chatC = llmclient.NewChatClient(s.APIBaseURL, s.APIKey, s.ChatModel)
	}

	kbSvc := kb.NewService(kbRepo, docRepo)
	docSvc := document.NewService(docRepo, st)
	chatSvc := chat.NewService(chatRepo, chunkRepo, docRepo, settingSvc, embedC, chatC, rerankC)

	w := worker.NewParseWorker(db, st, embedC, chunker.Split)

	return &Services{KB: kbSvc, Document: docSvc, Chat: chatSvc, Setting: settingSvc, Worker: w}, nil
}

func New(db *gorm.DB, cfg Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	if db == nil {
		return r
	}

	svcs, err := NewServices(db, cfg)
	if err != nil {
		panic(err)
	}

	kbH := handler.NewKBHandler(svcs.KB)
	api.POST("/kb", kbH.Create)
	api.GET("/kb", kbH.List)
	api.GET("/kb/:id", kbH.Get)
	api.PUT("/kb/:id", kbH.Update)
	api.DELETE("/kb/:id", kbH.Delete)

	docH := handler.NewDocumentHandler(svcs.Document, svcs.Worker)
	api.POST("/kb/:id/documents", docH.Upload)
	api.GET("/kb/:id/documents", docH.List)
	api.GET("/documents/:id", docH.Get)
	api.DELETE("/documents/:id", docH.Delete)
	api.POST("/documents/:id/reparse", docH.Reparse)

	chatH := handler.NewChatHandler(svcs.Chat)
	api.POST("/chat", chatH.Chat)
	api.GET("/sessions", chatH.ListSessions)
	api.GET("/sessions/:id/messages", chatH.GetMessages)
	api.DELETE("/sessions/:id", chatH.DeleteSession)

	setH := handler.NewSettingHandler(svcs.Setting)
	api.GET("/setting", setH.Get)
	api.PUT("/setting", setH.Update)

	svcs.Worker.Start()
	svcs.Worker.Recover()

	return r
}