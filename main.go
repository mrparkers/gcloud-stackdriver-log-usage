package main

import (
	"code.cloudfoundry.org/bytefmt"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/monitoring/v3"
	"os"
	"time"
)

var (
	startTime = time.Now().UTC().Add(-time.Hour * 2)
	endTime   = time.Now().UTC()
)

func createServices() (*monitoring.Service, *cloudresourcemanager.Service, error) {
	ctx := context.Background()

	client, err := google.DefaultClient(ctx, monitoring.MonitoringScope, cloudresourcemanager.CloudPlatformReadOnlyScope)

	if err != nil {
		return nil, nil, err
	}

	monitoringService, err := monitoring.New(client)

	if err != nil {
		return nil, nil, err
	}

	cloudResourceManagerService, err := cloudresourcemanager.New(client)

	if err != nil {
		return nil, nil, err
	}

	return monitoringService, cloudResourceManagerService, nil
}

func getListOfProjects(service *cloudresourcemanager.Service) ([]*cloudresourcemanager.Project, error) {
	response, err := service.Projects.List().Do()

	if err != nil {
		return nil, err
	}

	return response.Projects, nil
}

func getTargetLogIngestionValueForCurrentDay() (uint64, error) {
	const maxLogIngestionPerMonth = "50G"

	maxBytesPerMonth, err := bytefmt.ToBytes(maxLogIngestionPerMonth)

	if err != nil {
		return 0, err
	}

	now := time.Now()

	currentDay := now.Day()
	lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()

	percentage := float64(currentDay) / float64(lastDay)

	goalAmountOfBytes := float64(maxBytesPerMonth) * percentage

	return uint64(goalAmountOfBytes), nil
}

func getLogUsageByResourceForProject(projectId string, service *monitoring.Service) (map[string]uint64, error) {
	var totalBytes uint64

	logs := make(map[string]uint64)

	const metric = "logging.googleapis.com/billing/monthly_bytes_ingested"

	response, err := service.Projects.TimeSeries.List(fmt.Sprintf("projects/%s", projectId)).
		Filter(fmt.Sprintf(`metric.type="%s"`, metric)).
		IntervalStartTime(startTime.Format(time.RFC3339)).
		IntervalEndTime(endTime.Format(time.RFC3339)).
		Do()

	if err != nil {
		return nil, err
	}

	for _, series := range response.TimeSeries {
		metricProjectId := series.Resource.Labels["project_id"]

		if metricProjectId != projectId {
			continue
		}

		resourceType := series.Metric.Labels["resource_type"]
		int64Value := series.Points[0].Value.Int64Value
		numBytes := uint64(*int64Value)

		prevBytes, ok := logs[resourceType]

		if ok {
			logs[resourceType] = numBytes + prevBytes
		} else {
			logs[resourceType] = numBytes
		}

		totalBytes += numBytes
	}

	logs["TOTAL"] = totalBytes

	return logs, nil
}

func handleError(err error) {
	fmt.Println(err)
	os.Exit(1)
}

func main() {
	monitoringService, cloudResourceManagerService, err := createServices()

	if err != nil {
		handleError(fmt.Errorf("error creating required services: %s", err))
	}

	projects, err := getListOfProjects(cloudResourceManagerService)

	if err != nil {
		handleError(fmt.Errorf("error getting list of projects: %s", err))
	}

	targetBytes, err := getTargetLogIngestionValueForCurrentDay()

	if err != nil {
		handleError(fmt.Errorf("error getting the target log bytes value for today: %s", err))
	}

	for _, project := range projects {
		projectId := project.ProjectId

		if project.LifecycleState != "ACTIVE" {
			fmt.Printf("\nSkipping project %s due to project state %s\n", projectId, project.LifecycleState)

			continue
		}

		logs, err := getLogUsageByResourceForProject(projectId, monitoringService)

		if err != nil {
			handleError(fmt.Errorf("error getting log usage for project %s: %s", projectId, err))
		}

		totalBytes := logs["TOTAL"]

		if totalBytes == 0 {
			continue
		}

		fmt.Printf("\nProject %s\n", projectId)

		for resource, bytes := range logs {
			if resource == "TOTAL" {
				continue
			}

			fmt.Printf("%s - %s\n", resource, bytefmt.ByteSize(bytes))
		}

		fmt.Printf("TOTAL - %s\n", bytefmt.ByteSize(totalBytes))

		if totalBytes > targetBytes {
			fmt.Printf("[WARNING] Current log ingestion of %s is greater than the target value of %s, consider adding log exclusions\n", bytefmt.ByteSize(totalBytes), bytefmt.ByteSize(targetBytes))
		}
	}
}
