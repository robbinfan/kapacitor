package load

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ghodss/yaml"

	"github.com/influxdata/kapacitor/client/v1"
	"github.com/pkg/errors"
)

var defaultURL = "http://localhost:9092"

type HardError struct {
	err error
}

func (e HardError) Error() string {
	return e.err.Error()
}

type Service struct {
	mu     sync.Mutex
	config Config

	cli    *client.Client
	logger *log.Logger
}

func NewService(c Config, h http.Handler, l *log.Logger) (*Service, error) {
	cfg := client.Config{
		URL: defaultURL,
	}
	if h != nil {
		cfg.Transport = client.NewLocalTransport(h)
	}
	cli, err := client.New(cfg)

	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	return &Service{
		config: c,
		logger: l,
		cli:    cli,
	}, nil
}

func (s *Service) Open() error {
	return nil
}

func (s *Service) Close() error {
	return nil
}

// TaskFiles gets a slice of all files with the .tick file extension
// in the configured task directory.
func (s *Service) taskFiles() (tickscripts []string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasksDir := s.config.tasksDir()

	files, err := ioutil.ReadDir(tasksDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		switch ext := filepath.Ext(filename); ext {
		case ".tick":
			tickscripts = append(tickscripts, filepath.Join(tasksDir, filename))
		default:
			continue
		}
	}

	return
}

// TemplateFiles gets a slice of all files with the .tick file extension
// and any associated files with .json, .yml, and .yaml file extentions
// in the configured template directory.
func (s *Service) templateFiles() (tickscripts []string, tmplVars []string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	templatesDir := s.config.templatesDir()

	files, err := ioutil.ReadDir(templatesDir)
	if err != nil {
		return nil, nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		switch ext := filepath.Ext(filename); ext {
		case ".tick":
			tickscripts = append(tickscripts, filepath.Join(templatesDir, filename))
		case ".yml", ".json", ".yaml":
			tmplVars = append(tmplVars, filepath.Join(templatesDir, filename))
		default:
			continue
		}
	}

	return
}

// HandlerFiles gets a slice of all files with the .json, .yml, and
// .yaml file extentions in the configured handler directory.
func (s *Service) handlerFiles() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	handlers := []string{}

	handlersDir := s.config.handlersDir()

	files, err := ioutil.ReadDir(handlersDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		switch ext := filepath.Ext(filename); ext {
		case ".yml", ".json", ".yaml":
			handlers = append(handlers, filepath.Join(handlersDir, filename))
		default:
			continue
		}
	}

	return handlers, nil
}

func (s *Service) Load() error {
	if err := s.load(); err != nil && s.config.Hard {
		return HardError{err: err}
	} else {
		return err
	}
}

func (s *Service) load() error {
	if !s.config.Enabled {
		return nil
	}
	s.logger.Println("D! Loading tasks")
	err := s.loadTasks()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	s.logger.Println("D! Loading templates")
	err = s.loadTemplates()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	s.logger.Println("D! Loading handlers")
	err = s.loadHandlers()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (s *Service) loadTasks() error {
	files, err := s.taskFiles()
	if err != nil {
		return fmt.Errorf("failed to load tickscripts: %v", err)
	}

	for _, f := range files {
		s.logger.Println("D! Loading task from file: ", f)
		if err := s.loadTask(f); err != nil {
			return fmt.Errorf("failed to load file %s: %s", f, err.Error())
		}
	}

	return nil
}

func (s *Service) loadTask(f string) error {
	file, err := os.Open(f)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("failed to open file %v: %v", f, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file %v: %v", f, err)
	}

	script := string(data)
	fn := file.Name()
	id := strings.TrimSuffix(filepath.Base(fn), filepath.Ext(fn))

	l := s.cli.TaskLink(id)
	task, _ := s.cli.Task(l, nil)
	if task.ID == "" {
		o := client.CreateTaskOptions{
			ID:         id,
			TICKscript: script,
			Status:     client.Enabled,
		}

		if _, err := s.cli.CreateTask(o); err != nil {
			return fmt.Errorf("failed to create task: %v", err)
		}
	} else {
		o := client.UpdateTaskOptions{
			ID:         id,
			TICKscript: script,
		}
		if _, err := s.cli.UpdateTask(l, o); err != nil {
			return fmt.Errorf("failed to create task: %v", err)
		}

		// do reload
		_, err := s.cli.UpdateTask(l, client.UpdateTaskOptions{Status: client.Disabled})
		if err != nil {
			return err
		}
		_, err = s.cli.UpdateTask(l, client.UpdateTaskOptions{Status: client.Enabled})
		if err != nil {
			return err
		}

	}
	return nil
}

func (s *Service) loadTemplates() error {
	files, vars, err := s.templateFiles()
	if err != nil {
		return err
	}

	for _, f := range files {
		s.logger.Println("D! Loading template from file: ", f)
		if err := s.loadTemplate(f); err != nil {
			return fmt.Errorf("failed to load file %s: %s", f, err.Error())
		}
	}

	for _, v := range vars {
		s.logger.Println("D! Loading task vars from file: ", v)
		if err := s.loadVars(v); err != nil {
			return fmt.Errorf("failed to load file %s: %s", v, err.Error())
		}
	}
	return nil
}

func (s *Service) loadTemplate(f string) error {
	file, err := os.Open(f)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("failed to open file %v: %v", f, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file %v: %v", f, err)
	}

	script := string(data)
	fn := file.Name()
	id := strings.TrimSuffix(filepath.Base(fn), filepath.Ext(fn))

	l := s.cli.TemplateLink(id)
	task, _ := s.cli.Template(l, nil)
	if task.ID == "" {
		o := client.CreateTemplateOptions{
			ID:         id,
			TICKscript: script,
		}

		if _, err := s.cli.CreateTemplate(o); err != nil {
			return fmt.Errorf("failed to create template: %v", err)
		}
	} else {
		o := client.UpdateTemplateOptions{
			ID:         id,
			TICKscript: script,
		}
		if _, err := s.cli.UpdateTemplate(l, o); err != nil {
			return fmt.Errorf("failed to create template: %v", err)
		}
	}
	return nil
}

func (s *Service) loadVars(f string) error {
	file, err := os.Open(f)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("failed to open file %v: %v", f, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file %v: %v", f, err)
	}

	fn := file.Name()
	id := strings.TrimSuffix(filepath.Base(fn), filepath.Ext(fn))

	fileVars := client.TaskVars{}
	switch ext := path.Ext(f); ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &fileVars); err != nil {
			return errors.Wrapf(err, "failed to unmarshal yaml task vars file %q", f)
		}
	case ".json":
		if err := json.Unmarshal(data, &fileVars); err != nil {
			return errors.Wrapf(err, "failed to unmarshal json task vars file %q", f)
		}
	default:
		return errors.New("bad file extension. Must be YAML or JSON")
	}

	l := s.cli.TaskLink(id)
	task, _ := s.cli.Task(l, nil)
	if task.ID == "" {
		var o client.CreateTaskOptions
		o, err = fileVars.CreateTaskOptions()
		if err != nil {
			return fmt.Errorf("failed to initialize create task options: %v", err)
		}

		o.ID = id
		o.Status = client.Enabled
		if _, err := s.cli.CreateTask(o); err != nil {
			return fmt.Errorf("failed to create task: %v", err)
		}
	} else {
		var o client.UpdateTaskOptions
		o, err := fileVars.UpdateTaskOptions()
		if err != nil {
			return fmt.Errorf("failed to initialize create task options: %v", err)
		}

		o.ID = id
		if _, err := s.cli.UpdateTask(l, o); err != nil {
			return fmt.Errorf("failed to create task: %v", err)
		}
		// do reload
		_, err = s.cli.UpdateTask(l, client.UpdateTaskOptions{Status: client.Disabled})
		if err != nil {
			return err
		}
		_, err = s.cli.UpdateTask(l, client.UpdateTaskOptions{Status: client.Enabled})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) loadHandlers() error {
	files, err := s.handlerFiles()
	if err != nil {
		return err
	}

	for _, f := range files {
		s.logger.Println("D! Loading handler from file: ", f)
		if err := s.loadHandler(f); err != nil {
			return fmt.Errorf("failed to load file %s: %s", f, err.Error())
		}
	}
	return nil
}

func (s *Service) loadHandler(f string) error {
	file, err := os.Open(f)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("failed to open file %v: %v", f, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file %v: %v", f, err)
	}

	var o client.TopicHandlerOptions
	switch ext := path.Ext(f); ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &o); err != nil {
			return errors.Wrapf(err, "failed to unmarshal yaml task vars file %q", f)
		}
	case ".json":
		if err := json.Unmarshal(data, &o); err != nil {
			return errors.Wrapf(err, "failed to unmarshal json task vars file %q", f)
		}
	default:
		return errors.New("bad file extension. Must be YAML or JSON")
	}

	l := s.cli.TopicHandlerLink(o.Topic, o.ID)
	handler, _ := s.cli.TopicHandler(l)
	if handler.ID == "" {
		_, err := s.cli.CreateTopicHandler(s.cli.TopicHandlersLink(o.Topic), o)
		if err != nil {
			return err
		}
	} else {
		_, err := s.cli.ReplaceTopicHandler(l, o)
		if err != nil {
			return err
		}
	}

	return nil
}
