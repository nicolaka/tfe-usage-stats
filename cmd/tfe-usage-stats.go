package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/evandro-slv/go-cli-charts/bar"
	tfe "github.com/hashicorp/go-tfe"
	"github.com/peytoncasper/tfe-usage-stats/internal"
)

func main() {

	host := flag.String("host", "https://app.terraform.io", "TFE/C hostname")
	token := flag.String("token", "", "TFE/C API token")

	flag.Parse()

	if *token == "" {
		log.Println("API Token Not Provided")
		os.Exit(1)
	}

	config := &tfe.Config{
		Address: *host,
		Token:   *token,
	}

	client, err := tfe.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}

	orgs, err := internal.GetOrganizations(client)
	if err != nil {
		log.Println(err)
	}

	workspaces, err := internal.GetWorkspaces(client, orgs)

	if err != nil {
		log.Println(err)
	}

	teams, err := internal.GetTeams(client, orgs)

	if err != nil {
		log.Println(err)
	}

	runs, err := internal.GetRuns(client, workspaces)

	users := map[string]int{}
	for _, t := range teams {
		for _, u := range t.Users {
			if _, ok := users[u.ID]; ok {
				users[u.ID] += 1
			} else {
				users[u.ID] = 1
			}
		}
	}

	fmt.Printf("Total Users: %d\n", len(users)-1)
	fmt.Printf("Total Workspaces: %d\n", len(workspaces))

	data := make(map[string]float64)

	for k, v := range runs {
		data[k] = float64(len(v))
	}

	graph := bar.Draw(data, bar.Options{
		Chart: bar.Chart{
			Height: 10,
		},
		Bars: bar.Bars{
			Width: 10,
			Margin: bar.Margin{
				Left:  1,
				Right: 3,
			},
		},
		Precision: 1,
	})
	//

	histogram := make([]int64, 0)
	var runsum int64 = 0

	for _, m := range runs {
		for _, r := range m {
			t := r.StatusTimestamps.AppliedAt.Sub(r.StatusTimestamps.PlanQueuabledAt).Milliseconds()
			runsum += 1

			if len(histogram) > 0 {
				for i := range histogram {
					if histogram[i] > t {
						histogram = append(histogram, 0)
						copy(histogram[i+1:], histogram[i:])
						histogram[i] = t

						break
					} else if (i + 1) == len(histogram) {
						histogram = append(histogram, t)
					}
				}
			} else {
				histogram = append(histogram, t)
			}
		}
	}

	fmt.Printf("Total Successful Runs: %d\n", runsum)
	fmt.Printf("Succesful Applies per Month: \n")
	println(graph)

	count := len(histogram)
	var sum int64 = 0

	percentiles := make([]float64, 5)
	counts := make([]int, 5)

	for i, v := range histogram {
		sum += v

		if i == int(math.Floor(float64(count)*.5)) {
			percentiles[0] = float64(sum / int64(i))
			counts[0] = i
		}
		if i == int(math.Floor(float64(count)*.75)) {
			percentiles[1] = float64(sum / int64(i))
			counts[1] = i
		}
		if i == int(math.Floor(float64(count)*.90)) {
			percentiles[2] = float64(sum / int64(i))
			counts[2] = i
		}
		if i == int(math.Floor(float64(count)*.95)) {
			percentiles[3] = float64(sum / int64(i))
			counts[3] = i
		}
		if i == int(math.Floor(float64(count)*.99)) {
			percentiles[4] = float64(sum / int64(i))
			counts[4] = i
		}
	}

	barCharacter := "█"

	fmt.Printf("\nApply Execution Time Histogram: \n")

	fmt.Printf("p50 [ %4d / %4d ] %10.1fs: %s\n", counts[0], count, percentiles[0]/1000, strings.Repeat(barCharacter, int(math.Floor(percentiles[0]/(percentiles[4]/50)))))
	fmt.Printf("p75 [ %4d / %4d ] %10.1fs: %s\n", counts[1], count, percentiles[1]/1000, strings.Repeat(barCharacter, int(math.Floor(percentiles[1]/(percentiles[4]/50)))))
	fmt.Printf("p90 [ %4d / %4d ] %10.1fs: %s\n", counts[2], count, percentiles[2]/1000, strings.Repeat(barCharacter, int(math.Floor(percentiles[2]/(percentiles[4]/50)))))
	fmt.Printf("p95 [ %4d / %4d ] %10.1fs: %s\n", counts[3], count, percentiles[3]/1000, strings.Repeat(barCharacter, int(math.Floor(percentiles[3]/(percentiles[4]/50)))))
	fmt.Printf("p99 [ %4d / %4d ] %10.1fs: %s\n", counts[4], count, percentiles[4]/1000, strings.Repeat(barCharacter, int(math.Floor(percentiles[4]/(percentiles[4]/50)))))

	versionMatrix := map[string]map[string]int{}

	majorVersions := make([]string, 0)
	minorVersions := make([]string, 0)

	for _, workspace := range workspaces {
		versionParts := strings.Split(workspace.TerraformVersion, ".")

		majorVersion := versionParts[0] + "." + versionParts[1]
		minorVersion := versionParts[2]

		majorVersions = appendVersion(majorVersion, majorVersions, "down")
		minorVersions = appendVersion(minorVersion, minorVersions, "up")

		if x, ok := versionMatrix[majorVersion]; ok {
			x[minorVersion] = x[minorVersion] + 1
		} else {
			versionMatrix[majorVersion] = map[string]int{
				minorVersion: 1,
			}

		}
	}

	fmt.Printf("\n%7s\n", "Version Matrix:")

	for _, minorVersion := range minorVersions {
		fmt.Printf("%6s", "."+minorVersion+" |")
		for _, majorValue := range majorVersions {
			if count, ok := versionMatrix[majorValue][minorVersion]; ok {
				fmt.Printf("%6d", count)
			} else {
				fmt.Printf("%6s", " ")
			}
		}
		fmt.Printf("\n")
	}

	fmt.Printf("%6s", " ")
	for _, v := range majorVersions {
		fmt.Printf("%6s", v)
	}

}

func appendVersion(version string, versions []string, sortDirection string) []string {
	if len(versions) == 0 {
		versions = append(versions, version)
	}

	for i, v := range versions {
		if version == v {
			return versions
		}

		if sortDirection == "up" {
			v1, _ := strconv.ParseFloat(v, 64)
			v2, _ := strconv.ParseFloat(version, 64)

			if v1 < v2 {
				versions = append(versions, "")
				copy(versions[i+1:], versions[i:])
				versions[i] = version

				break
			} else if (i + 1) == len(versions) {
				versions = append(versions, version)
			}
		} else if sortDirection == "down" {
			v1, _ := strconv.ParseFloat(v, 64)
			v2, _ := strconv.ParseFloat(version, 64)

			if v1 > v2 {
				versions = append(versions, "")
				copy(versions[i+1:], versions[i:])
				versions[i] = version

				break
			} else if (i + 1) == len(versions) {
				versions = append(versions, version)
			}
		}

	}

	return versions
}
