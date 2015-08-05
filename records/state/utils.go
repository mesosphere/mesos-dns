package state

import (
	"strconv"
	"strings"
)

// returns an array of ports from a range
func yankPorts(ports string) []string {
	rhs := strings.Split(ports, "[")[1]
	lhs := strings.Split(rhs, "]")[0]

	yports := []string{}

	mports := strings.Split(lhs, ",")
	for _, port := range mports {
		tmp := strings.TrimSpace(port)
		pz := strings.Split(tmp, "-")
		lo, _ := strconv.Atoi(pz[0])
		hi, _ := strconv.Atoi(pz[1])

		for t := lo; t <= hi; t++ {
			yports = append(yports, strconv.Itoa(t))
		}
	}
	return yports
}

// ForEachPort calls the given function f for each defined port in the given task
func (t *Task) ForEachPort(f func(port string)) {
	if t.Resources.Ports != "" {
		ports := yankPorts(t.Resources.Ports)
		for _, port := range ports {
			f(port)
		}
	}
}

// ContainerIP extracts a container ip from a Mesos state.json task. If not
// container ip is provided, an empty string is returned.
func (t *Task) ContainerIP() string {
	const containerIPTaskStatusLabel = "Docker.NetworkSettings.IPAddress"

	// find TASK_RUNNING statuses
	var latestContainerIP string
	var latestTimestamp float64
	for _, status := range t.Statuses {
		if status.State != "TASK_RUNNING" {
			continue
		}

		// find the latest docker-inspect label
		for _, label := range status.Labels {
			if label.Key == containerIPTaskStatusLabel && status.Timestamp > latestTimestamp {
				latestContainerIP = label.Value
				latestTimestamp = status.Timestamp
				break
			}
		}
	}

	return latestContainerIP
}
