package main

import (
	"bufio"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

type EngineConfiguration struct {
	Port                int
	Command, CommandAlt string
}

func ConnectToRDSInstance(cfg Configuration, filter string, args []string) {
	channel := make(chan []*rds.DBInstance)
	regions := cfg.AWS.Regions
	for _, region := range regions {
		go func(region string) {
			config := getAwsConfig(region)
			svc := rds.New(session.New(&config))
			resp, err := svc.DescribeDBInstances(&rds.DescribeDBInstancesInput{})
			if err != nil {
				log.Fatal(err)
			}
			rows := []*rds.DBInstance{}
			for _, i := range resp.DBInstances {
				if strings.Contains(*i.Endpoint.Address, filter) {
					rows = append(rows, i)
				}
			}
			channel <- rows
		}(region)
	}

	var instances []*rds.DBInstance
	for i := 0; i < len(regions); i++ {
		instances = append(instances, <-channel...)
	}
	close(channel)

	if len(instances) == 0 {
		log.Fatal("No instances found.")
	} else if len(instances) == 1 {
		connectToRDSInstance(instances[0], args, cfg)
	} else {
		table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(table, "#\tName\tEndpoint\tEngine")
		for i, instance := range instances {
			name := strings.Split(*instance.Endpoint.Address, ".")[0]
			row := []string{strconv.Itoa(i), name, *instance.Endpoint.Address, *instance.Engine}
			fmt.Fprintln(table, strings.Join(row, "\t"))
		}
		table.Flush()

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Fprint(os.Stderr, "Enter a valid instance number: ")
			result, err := reader.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			i, err := strconv.Atoi(strings.Trim(result, "\n"))
			if err == nil && len(instances) > i {
				connectToRDSInstance(instances[i], args, cfg)
				break
			}
		}
	}
}

func getRDSConfig(endpoint string, credentials []RDSConfiguration) RDSConfiguration {
	for _, i := range credentials {
		if strings.HasPrefix(endpoint, i.Prefix) {
			if i.Database == "" {
				segments := strings.Split(endpoint, "-")[1]
				if len(segments) > 1 {
					i.Database = strings.Split(segments, ".")[0]
				}
			}
			return i
		}
	}
	log.Fatalf("No RDS Configuration found for %s, please check your configuration. Run: 'bub config'", endpoint)
	return RDSConfiguration{}
}

func getEnvironment(endpoint string, environments []Environment) Environment {
	for _, i := range environments {
		if strings.HasPrefix(endpoint, i.Name) {
			return i
		}
	}
	log.Fatalf("No environment matched %s, please check your configuration. Run: 'bub config'", endpoint)
	return Environment{}
}

func tunnelIsReady(port int) bool {
	_, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%v", port))
	if err != nil {
		return false
	}
	return true
}

func random(min, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}

func getEngineConfiguration(engine string) EngineConfiguration {
	if engine == "mysql" {
		return EngineConfiguration{3306, "mycli", "mysql"}
	}
	return EngineConfiguration{5432, "pgcli", "psql"}
}

// Escape codes for iTerm2
func setBackground(endpoint string) {
	if strings.HasPrefix(endpoint, "prod") {
		// red for production
		print("\033]Ph501010\033\\")
	} else {
		// yellow for staging
		print("\033]Ph403010\033\\")
	}
}

func rdsCleanup(tunnel *exec.Cmd) {
	// green for safe
	print("\033]Ph103010\033\\")
	tunnel.Process.Kill()
}
func connectToRDSInstance(instance *rds.DBInstance, args []string, cfg Configuration) {
	endpoint := *instance.Endpoint.Address
	jump := getEnvironment(endpoint, cfg.AWS.Environments).Jumphost
	rdsConfig := getRDSConfig(endpoint, cfg.AWS.RDS)
	port := random(40000, 60000)
	engine := getEngineConfiguration(*instance.Engine)

	tunnelPath := fmt.Sprintf("%v:%v:%v", port, endpoint, engine.Port)
	log.Printf("Connecting to %s through %s", tunnelPath, jump)
	tunnel := exec.Command("ssh", "-NL", tunnelPath, jump)
	tunnel.Stderr = os.Stderr
	err := tunnel.Start()

	log.Print("Waiting for tunnel...")
	for {
		time.Sleep(100 * time.Millisecond)
		if tunnelIsReady(port) {
			break
		}
	}

	env := []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		"PGHOST=127.0.0.1",
		"MYSQL_HOST=127.0.0.1",
		"PGDATABASE=" + rdsConfig.Database,
		"PGUSER=" + rdsConfig.User,
		"PGPASSWORD=" + rdsConfig.Password,
		"MYSQL_PWD=" + rdsConfig.Password,
		fmt.Sprintf("PGPORT=%v", port),
		fmt.Sprintf("MYSQL_TCP_PORT=%v", port)}

	command := ""
	if len(args) == 0 {
		command, err = exec.LookPath(engine.Command)
		if err != nil {
			command, err = exec.LookPath(engine.CommandAlt)
			if err != nil {
				log.Fatalf("Install %s and/or %s.", engine.Command, engine.CommandAlt)
			}
		}
	} else {
		if args[0] == "--" {
			args = args[1:]
		}
		command = args[0]
		args = args[1:]
	}

	isDefaultCommand := strings.Contains(command, engine.Command) || strings.Contains(command, engine.CommandAlt)
	if *instance.Engine == "mysql" && isDefaultCommand {
		args = append(args, fmt.Sprintf("-u'%s'", rdsConfig.User), rdsConfig.Database)
	}

	log.Printf("Running: %s %s", command, strings.Join(args, " "))
	setBackground(endpoint)
	cmd := exec.Command(command, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		rdsCleanup(tunnel)
		log.Fatal(err)
	}
	rdsCleanup(tunnel)
}