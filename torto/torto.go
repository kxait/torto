package torto

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	target                 string
	argReplacements        map[string]string
	debug                  bool
	force                  bool
	targetsFilePath        string
	defaultTargetsFilePath string
	thisTarget             []string
}

type Targets struct {
	Targets map[string][]string `yaml:"targets"`
	Vars    map[string]string   `yaml:"vars"`
}

func dbgln(c *Config, format string, a ...any) {
	if c.debug {
		fmt.Printf(format, a...)
	}
}

func getTargetsFile(filename string) (*Targets, error) {
	jsonFile, err := os.Open(filename)

	if err != nil {
		return nil, err
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var targets Targets

	err = yaml.Unmarshal(byteValue, &targets)

	if err != nil {
		return nil, err
	}

	return &targets, nil
}

func emptyTargets() *Targets {
	return &Targets{
		Targets: make(map[string][]string),
		Vars:    make(map[string]string),
	}
}

func getTargets(c *Config) (*Targets, error) {
	defaultTargets, defaultErr := getTargetsFile(c.defaultTargetsFilePath)
	thisDirTargets, thisDirErr := getTargetsFile(c.targetsFilePath)

	if defaultErr != nil && thisDirErr != nil {
		return nil, thisDirErr
	}

	if defaultTargets == nil {
		defaultTargets = emptyTargets()
	}

	if thisDirTargets == nil {
		thisDirTargets = emptyTargets()
	}

	for k, v := range thisDirTargets.Targets {
		defaultTargets.Targets[k] = v
	}
	for k, v := range thisDirTargets.Vars {
		defaultTargets.Vars[k] = v
	}

	for k, v := range defaultTargets.Vars {
		c.argReplacements[k] = v
	}

	return defaultTargets, nil
}

func getTargetNameAndRunArgs(programArgs []string) (string, map[string]string, error) {
	runArgs := make(map[string]string)
	nonValueReplacementArgs := []string{}

	r, _ := regexp.Compile("^([a-zA-Z0-9]+)=(.+)$")

	for _, arg := range programArgs {
		matches := r.FindStringSubmatch(arg)
		if len(matches) > 0 {
			var value string
			if matches[2] != "" {
				value = matches[2]
			} else {
				value = matches[3]
			}

			runArgs[matches[1]] = value
		} else {
			nonValueReplacementArgs = append(nonValueReplacementArgs, arg)
		}
	}

	if len(nonValueReplacementArgs) == 0 {
		return "", nil, errors.New("target missing. check usage with -h")
	}

	target := nonValueReplacementArgs[0]

	cmdVar := strings.Join(nonValueReplacementArgs[1:], " ")
	runArgs["CMD"] = cmdVar

	if path, err := os.Executable(); err != nil {
		return "", nil, err
	} else {
		runArgs["BIN"] = path
	}

	return target, runArgs, nil
}

func withResolvedArgs(argValue string, c *Config) string {
	for k, v := range c.argReplacements {
		if strings.Contains(argValue, "$"+k) {
			resolvedArgValue := withResolvedArgs(v, c)
			argValue = strings.ReplaceAll(argValue, "$"+k, resolvedArgValue)
		}
	}
	return argValue
}

func Execute(c *Config) error {
	var executor string
	if runtime.GOOS == "windows" {
		executor = "powershell"
	} else {
		executor = "sh"
	}

	configYml, _ := yaml.Marshal(&c)
	dbgln(c, "%s", string(configYml))

	for _, v := range c.thisTarget {
		command := withResolvedArgs(v, c)

		cmd := exec.Command(executor, "-c", command)

		if c.debug {
			fmt.Println(cmd.String())
			continue
		}

		stderr := &bytes.Buffer{}

		cmd.Stdout = os.Stdout
		cmd.Stderr = stderr

		err := cmd.Run()

		if err != nil {
			var f func(a ...any)
			if strings.HasSuffix(stderr.String(), "\n") {
				f = func(a ...any) { fmt.Print(a...) }
			} else {
				f = func(a ...any) { fmt.Println(a...) }
			}

			f(c.target + ": error executing '" + command + "': " + stderr.String())
			if !c.force {
				return err
			} else {
				fmt.Println(err)
			}
		}

	}

	return nil
}

func CreateCommand() *cobra.Command {
	config := &Config{
		force:                  false,
		argReplacements:        make(map[string]string),
		targetsFilePath:        "torto.yml",
		defaultTargetsFilePath: "~/torto.yml",
		target:                 "",
		thisTarget:             []string{},
	}

	command := &cobra.Command{
		Use:          `torto target [args]`,
		Example:      `torto hello_world VAR1=test"`,
		Args:         ArgsValidator(config),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Execute(config)
		},
	}

	command.PersistentFlags().BoolVarP(&config.force, "force", "f", false, "run all commands regardless of error")
	command.PersistentFlags().BoolVarP(&config.debug, "debug", "d", false, "runs in debug mode")

	return command
}

func ArgsValidator(c *Config) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		targetName, runArgs, err := getTargetNameAndRunArgs(args)
		if err != nil {
			return err
		}

		c.argReplacements = runArgs
		c.target = withResolvedArgs(targetName, c)

		targets, err := getTargets(c)

		if err != nil {
			return err
		}

		thisTarget, ok := targets.Targets[targetName]

		if !ok {
			return errors.New("target " + targetName + " does not exist")
		}

		c.thisTarget = thisTarget

		return nil
	}
}
