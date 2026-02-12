package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/manifoldco/promptui"
)

type Alarm struct {
	ID           int
	Label        string
	Time         time.Time
	Acknowledged bool
}

func getDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
		home = ""
	}
	return filepath.Join(home, ".alarms.db")
}

var dbFile string = getDBPath()

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: alarm <command>")
		return
	}

	initDB()

	switch os.Args[1] {
	case "help":
		printHelp()
	case "list":
		listAlarms()
	case "create":
		createAlarm()
	case "get":
		// Vérification de l'option '--tmux'
		tmux := false
		if len(os.Args) > 2 && os.Args[2] == "--tmux" {
			tmux = true
		}
		getPendingAlarms(tmux)
	case "ack":
		if len(os.Args) < 3 {
			fmt.Println("Usage: alarm ack <id>")
			return
		}
		id, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Println("Invalid ID format")
			return
		}
		acknowledgeAlarm(id)
	default:
		fmt.Println("Unknown command. Use 'alarm help' for usage information.")
	}
}

func printHelp() {
	fmt.Println(`Usage:
	alarm help          - Show help
	alarm list          - List all upcoming alarms
	alarm create        - Create a new alarm
	alarm get [--tmux]  - List non-acknowledged alarms due now
	alarm ack <id>      - Acknowledge an alarm by ID`)
}

func initDB() {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS alarms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		label TEXT,
		time TEXT,
		acknowledged BOOLEAN
	)`)
	if err != nil {
		log.Fatal(err)
	}
}

func listAlarms() {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, label, time, acknowledged FROM alarms")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var alarm Alarm
		var timeStr string
		err := rows.Scan(&alarm.ID, &alarm.Label, &timeStr, &alarm.Acknowledged)
		if err != nil {
			log.Fatal(err)
		}
		alarm.Time, _ = time.Parse(time.RFC3339, timeStr)
		fmt.Printf("ID: %d, Time: %s, Label: %s, Ack: %v\n", alarm.ID, alarm.Time, alarm.Label, alarm.Acknowledged)
	}
}

func createAlarm() {
	labelPrompt := promptui.Prompt{
		Label: "Enter label (optional)",
	}
	label, _ := labelPrompt.Run()

	// Obtenez la date actuelle
	currentDate := time.Now().Format("2006-01-02")

	// Préremplissez le champ de saisie de l'heure avec la date actuelle
	timePrompt := promptui.Prompt{
		Label:    "Enter time (YYYY-MM-DD HH:MM) or minutes from now",
		Default:  currentDate + " ",
		Validate: validateTimeInput,
	}
	timeInput, err := timePrompt.Run()
	if err != nil {
		log.Fatal(err)
	}

	var alarmTime time.Time
	if minutes, err := strconv.Atoi(timeInput); err == nil {
		alarmTime = time.Now().Add(time.Duration(minutes) * time.Minute)
	} else {
		alarmTime, err = time.Parse("2006-01-02 15:04", timeInput)
		if err != nil {
			fmt.Println("Invalid time format.")
			return
		}
	}

	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO alarms (label, time, acknowledged) VALUES (?, ?, ?)", label, alarmTime.Format(time.RFC3339), false)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Alarm created successfully!")
}

func validateTimeInput(input string) error {
	// Vérifiez si l'entrée est un nombre (minutes à partir de maintenant)
	if _, err := strconv.Atoi(input); err == nil {
		return nil
	}

	// Vérifiez si l'entrée est au format "YYYY-MM-DD HH:MM"
	_, err := time.Parse("2006-01-02 15:04", input)
	if err != nil {
		return fmt.Errorf("invalid time format")
	}
	return nil
}

func getPendingAlarms(tmux bool) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	rows, err := db.Query("SELECT id, label, time FROM alarms WHERE acknowledged = 0 AND time <= ?", now)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	if tmux {
		// Récupération des libellés pour l'affichage en mode tmux
		var labels []string
		count := 0
		for rows.Next() {
			var alarm Alarm
			var timeStr string
			err := rows.Scan(&alarm.ID, &alarm.Label, &timeStr)
			if err != nil {
				log.Fatal(err)
			}
			labels = append(labels, alarm.Label)
			count++
		}
		if count == 0 {
		        fmt.Printf("None ")
		} else{
			fmt.Printf("#[bg=red]%d alerts: %s#[bg=default]", count, strings.Join(labels, ", "))
		}
	} else {
		// Affichage classique
		for rows.Next() {
			var alarm Alarm
			var timeStr string
			err := rows.Scan(&alarm.ID, &alarm.Label, &timeStr)
			if err != nil {
				log.Fatal(err)
			}
			alarm.Time, _ = time.Parse(time.RFC3339, timeStr)
			fmt.Printf("Id: %d, Date: %s: %s\n", alarm.ID, alarm.Time, alarm.Label)
		}
	}
}

func acknowledgeAlarm(id int) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if id == 0 {
		now := time.Now().Format(time.RFC3339)
		res, err := db.Exec("UPDATE alarms SET acknowledged = 1 WHERE acknowledged = 0 AND time <= ?", now)
		if err != nil {
			log.Fatal(err)
		}

		n, _ := res.RowsAffected()
		if n == 0 {
			fmt.Println("Alarm not found.")
		} else {
			fmt.Println("Alarm acknowledged successfully!")
		}
	}else {
		res, err := db.Exec("UPDATE alarms SET acknowledged = 1 WHERE id = ?", id)
		if err != nil {
			log.Fatal(err)
		}

		n, _ := res.RowsAffected()
		if n == 0 {
			fmt.Println("Alarm not found.")
		} else {
			fmt.Println("Alarm acknowledged successfully!")
		}
	}

}

