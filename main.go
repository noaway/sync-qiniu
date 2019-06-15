package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"

	"github.com/fsnotify/fsnotify"
)

var (
	home = os.Getenv("HOME")
	dir  = ""
)

func main() {
	flag.StringVar(&dir, "dir", "", "sync dir")
	flag.Parse()

	if dir == "" {
		log.Println("dir not empty")
		return
	}

	watcher, err := monitor(dir, notify)
	if err != nil {
		log.Println(err)
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, []os.Signal{syscall.SIGINT, syscall.SIGTERM}...)
	<-signalChan
	watcher.Close()
}

func monitor(dir string, fn func(*fsnotify.Event), notifys ...fsnotify.Op) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if len(notifys) == 0 {
					fn(&event)
				} else {
					for _, notify := range notifys {
						if event.Op&notify != 0 {
							fn(&event)
							break
						}
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, err
	}
	return watcher, nil
}

func notify(event *fsnotify.Event) {
	cmd := home + "/.qshell/qshellupload.sh"
	bucket, err := uploadFile()
	if err != nil {
		log.Println(err)
		return
	}
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		syncqiniu(cmd, "create")
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		syncqiniu(cmd, "delete", bucket, path.Base(event.Name))
	}
}

func syncqiniu(name string, arg ...string) {
	command := exec.Command(name, arg...)
	err := command.Start()
	if nil != err {
		log.Println(err)
		return
	}
	err = command.Wait()
	if nil != err {
		log.Println(err)
	}
}

func uploadFile() (string, error) {
	f, err := os.Open(home + "/.qshell/upload.conf")
	if err != nil {
		return "", err
	}
	defer f.Close()

	ret := struct {
		Bucket string `json:"bucket"`
	}{}
	if err := json.NewDecoder(f).Decode(&ret); err != nil {
		return "", err
	}
	return ret.Bucket, nil
}
