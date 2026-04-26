package messagebroker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
)

type Credentials struct {
	login    string
	password string
}

type Consumer struct {
	db *sql.DB
}

func NewConsumer(db *sql.DB) *Consumer {
	return &Consumer{
		db: db,
	}
}

func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	log.Println("Consumer setup - инициализация")
	return nil
}

func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	log.Println("Consumer cleanup - очистка ресурсов")
	return nil
}

func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				log.Printf("Канал сообщений закрыт для партиций %d", claim.Partition())
				return nil
			}

			log.Printf("Получено сообщение: Топик=%s, Партиция=%d, Офсет=%d, Ключ=%s, Значение=%s\n",
				msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))
			var dataMap map[string]string
			if err := json.Unmarshal(msg.Value, &dataMap); err != nil {

				session.MarkMessage(msg, "")
				continue
			}

			if err := c.saveToDatabase(dataMap); err != nil {
				log.Printf("Ошибка сохранения в БД: %v", err)
				continue
			}

			log.Printf("Успешно сохранено %d записей в БД:", len(dataMap))
			session.MarkMessage(msg, "")

		case <-session.Context().Done():
			log.Printf("Session context done for partition %d", claim.Partition())
			return nil
		}
	}
}

func (c *Consumer) saveToDatabase(credentials map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
				INSERT INTO credentials (login, password)
				VALUES ($1, $2)
				ON CONFLICT (login) DO UPDATE SET
				                                  password = EXCLUDED.password
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for login, password := range credentials {
		_, err = stmt.ExecContext(ctx, login, password)
		if err != nil {
			log.Printf("Ошибка сохранения для логина %s: %v", login, err)
			return err
		}
		log.Printf("Сохарнено: логин=%s, пароль=%s", login, password)
	}

	return tx.Commit()
}

func CreateKafkaConsumer(db *sql.DB, brokers, certContent, keyContent, caContent, groupID, topic string) {
	cert, err := tls.X509KeyPair([]byte(certContent), []byte(keyContent))
	if err != nil {
		log.Fatalf("Ошибка загрузки сертичикатов: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(caContent)) {
		log.Fatalf("Ошибка доавления СА сертифиаката")
	}

	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true

	config.Net.TLS.Enable = true
	config.Net.TLS.Config = &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	consumerGroup, err := sarama.NewConsumerGroup([]string{brokers}, groupID, config)
	if err != nil {
		log.Fatalf("Ошибка создания consumer group: %v", err)
	}

	consumer := NewConsumer(db)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
		<-sigterm
		log.Println("Получен сигнал завершения, останавливаем consumer ...")
		cancel()
		consumerGroup.Close()
	}()

	go func() {
		for {
			if err := consumerGroup.Consume(ctx, []string{topic}, consumer); err != nil {
				log.Printf("Ошибка consumer: %v", err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
	log.Printf("Kafka consumer запущен. Топик: %s, Группа: %s", topic, groupID)

}
