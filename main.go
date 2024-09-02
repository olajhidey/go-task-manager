package main

import (
	"container/heap"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

type taskHeap []*Task

func (h taskHeap) Len() int           { return len(h) }
func (h taskHeap) Less(i, j int) bool { return h[i].RunAt.Before(h[j].RunAt) }
func (h taskHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *taskHeap) Push(x interface{}) {
	*h = append(*h, x.(*Task))
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type Scheduler struct {
	tasks taskHeap
	mu    sync.RWMutex
	stop  chan struct{}
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks: make(taskHeap, 0),
		stop:  make(chan struct{}),
	}
}

func (s *Scheduler) AddTask(task *Task) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	heap.Push(&s.tasks, task)
	log.Println("Task has been added!")
}

func (s *Scheduler) RemoveTask() *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task := heap.Pop(&s.tasks).(*Task)
	return task
}

func UpdateTaskFromDb(task *Task) error {
	updateQuery := `UPDATE tasks SET name = ?, description = ?, runAt = ?, status = ? WHERE id = ?`
	result, err := database.Exec(updateQuery, task.Name, task.Description, task.RunAt.Format("2006-01-02 15:04"), "Completed", task.ID)

	rowsAffected, err := result.RowsAffected()
	if rowsAffected != 0 {
		log.Println("Task updated successfully!")
	}
	return err
}

func (s *Scheduler) Start() {
	go func() {
		for {
			now := ParseDate(time.Now().Format("2006-01-02 15:04"))

			if len(s.tasks) == 0 {
				time.Sleep(1 * time.Second)
				continue
			}

			nextTask := s.tasks[0]
			if now.After(nextTask.RunAt) || now.Equal(nextTask.RunAt) {
				s.mu.RLock()
				task := heap.Pop(&s.tasks).(*Task) // Get the next task
				log.Println("This has been deleted")
				go s.executeTask(task)
				log.Println("Pronto!")
			}

			time.Sleep(1 * time.Second)
			// log.Println(now)
			// s.mu.RLock()
			// if len(s.tasks) == 0 {
			// 	s.mu.RUnlock()
			// 	time.Sleep(1 * time.Second)
			// 	continue
			// }

			// for _, task := range s.tasks{

			// 	// log.Println(task.Name, "is ready? ", now.After(task.RunAt) || now.Equal(task.RunAt))
			// 	if now.After(task.RunAt) || now.Equal(task.RunAt) {
			// 	deletedTask := s.RemoveTask()
			// 	log.Println("This has been deleted")
			// 	go s.executeTask(deletedTask)
			// 	log.Println("Pronto!")
			// }
			// }

			// nextTask := s.tasks[0]

			// if now.Before(nextTask.RunAt) {
			// 	s.mu.RUnlock()
			// 	time.Sleep(time.Until(nextTask.RunAt))
			// 	continue
			// }

			// nextTask := s.tasks[0]
			// now := time.Now()
			// if now.Before(nextTask.RunAt) {
			// 	s.mu.RUnlock()
			// 	time.Sleep(time.Until(nextTask.RunAt)) // Wait until next task is due
			// 	continue
			// }

			// if now.After(nextTask.RunAt) || now.Equal(nextTask.RunAt) {
			// 	s.mu.RUnlock()
			// 	s.mu.Lock()
			// 	task := heap.Pop(&s.tasks).(*Task) // Get the next task
			// 	log.Println("This has been deleted")
			// 	s.mu.Unlock()
			// 	go s.executeTask(task)
			// 	log.Println("Pronto!")
			// }

			// time.Sleep(1 * time.Second)

			// select {
			// case <-s.stop:
			// 	return
			// default:
			// }
		}
	}()

}

func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) executeTask(task *Task) {
	log.Printf("Executing task: %s|%v\n", task.Name, task.RunAt)

	if err := UpdateTaskFromDb(task); err != nil {
		log.Println("Error updating task in database: %v", err)
	}
}

func ParseDate(date string) time.Time {
	newDate := strings.Replace(date, "T", " ", 1)
	parsedTime, err := time.Parse("2006-01-02 15:04", newDate)
	if err != nil {
		fmt.Println("Error parsing time:", err)
	}
	return parsedTime
}

type Task struct {
	ID          int
	Name        string
	Description string
	RunAt       time.Time
	Status      string
}

type SchemaTask struct {
	Id          int    `json:"id, omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RunAt       string `json:"runAt"`
	Status      string `json:"status"`
}

var database *sql.DB
var scheduler *Scheduler

func CreateTask(ctx *gin.Context) {
	var task SchemaTask

	if err := ctx.ShouldBindBodyWithJSON(&task); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"Error": err.Error(),
		})
		return
	}

	// Insert data into DB
	insertTaskSQL := `INSERT INTO tasks (name, description, runAt, status) VALUES (?,?,?,?)`
	statement, err := database.Prepare(insertTaskSQL)

	if err != nil {
		panic(err)
	}
	result, err := statement.Exec(task.Name, task.Description, task.RunAt, task.Status)

	if err != nil {
		log.Fatal(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"Error": err.Error(),
		})
		return
	}

	log.Println("Task has been inserted!", task)

	lastInsertedId, err := result.LastInsertId()

	if err != nil {
		panic(err)
	}

	scheduler.AddTask(&Task{
		ID:          int(lastInsertedId),
		Name:        task.Name,
		Description: task.Description,
		RunAt:       ParseDate(task.RunAt),
		Status:      task.Status,
	})

	ctx.JSON(http.StatusOK, gin.H{
		"id":          lastInsertedId,
		"name":        task.Name,
		"description": task.Description,
		"runAt":       task.RunAt,
		"status":      task.Status,
	})
}

func GetTasks(ctx *gin.Context) {

	var tasks []SchemaTask

	allTasksQuery := "SELECT id, name, description, runAt, status FROM tasks"
	row, err := database.Query(allTasksQuery)

	if err != nil {
		panic(err)
	}

	defer row.Close()

	for row.Next() {
		var id int
		var name string
		var description string
		var runAt string
		var status string

		if err = row.Scan(&id, &name, &description, &runAt, &status); err != nil {

			log.Fatal(err)

			ctx.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})

			return
		}

		tasks = append(tasks, SchemaTask{
			Id:          id,
			Name:        name,
			Description: description,
			RunAt:       runAt,
			Status:      status,
		})
	}

	ctx.JSON(http.StatusOK, tasks)
}

func setUpSQLite() *sql.DB {
	database, err := sql.Open("sqlite3", "./task.db")
	if err != nil {
		panic(err)
	}

	log.Println("Database created!")

	return database
}

func createTable(database *sql.DB) {
	createTableSQL := ` CREATE TABLE IF NOT EXISTS tasks (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		 "name" TEXT,
		 "description" TEXT,
		 "runAt" TEXT,
		 "status" TEXT
	)
	`
	statement, err := database.Prepare(createTableSQL)
	if err != nil {
		log.Fatal(err.Error())
		panic(err)
	}

	statement.Exec()
	log.Println("Tasks table created")
}

func getAllTasks(database *sql.DB) []SchemaTask {
	var tasks []SchemaTask

	allTaskQuery := "SELECT id, name, description, runAt, status FROM tasks"
	rows, err := database.Query(allTaskQuery)

	if err != nil {
		panic(err)
	}

	defer rows.Close()

	for rows.Next() {
		var id int
		var name string
		var description string
		var runAt string
		var status string

		if err = rows.Scan(&id, &name, &description, &runAt, &status); err != nil {
			log.Println(err.Error())
		}

		tasks = append(tasks, SchemaTask{
			Id:          id,
			Name:        name,
			Description: description,
			RunAt:       runAt,
			Status:      status,
		})
	}

	return tasks
}

func DeleteTask(ctx *gin.Context) {
	id := ctx.Param("id")
	deleteQuery := "DELETE FROM tasks WHERE id = ?"
	results, err := database.Exec(deleteQuery, id)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	rowsAffected, err := results.RowsAffected()

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	if rowsAffected == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{
			"message": "Task not found",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Task deleted successfully",
	})

}
func main() {

	database = setUpSQLite()

	scheduler = NewScheduler()

	createTable(database)

	tasks := getAllTasks(database)

	for i := range tasks {
		task := tasks[i]
		if task.Status == "Scheduled" {
			scheduler.AddTask(&Task{
				ID:          task.Id,
				Name:        task.Name,
				Description: task.Description,
				RunAt:       ParseDate(task.RunAt),
				Status:      task.Status,
			})
		}
	}

	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file")
	}

	router := gin.Default()

	router.Static("/static", "./www")
	router.GET("/", func(ctx *gin.Context) {
		ctx.File("./www/index.html")
	})

	api := router.Group("/api")

	api.POST("/task/create", CreateTask)
	api.GET("/tasks", GetTasks)
	api.GET("/task/:id", func(ctx *gin.Context) {})
	api.PUT("/task/update/:id", func(ctx *gin.Context) {})
	api.DELETE("/task/delete/:id", DeleteTask)

	scheduler.Start()

	port := os.Getenv("PORT")

	router.Run(":" + port)

	select {}
}
