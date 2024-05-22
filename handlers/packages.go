package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/a-h/templ"
	"github.com/google/go-github/github"
	"github.com/oklog/ulid"
	"github.com/saintmalik/delta/model"
	"github.com/saintmalik/delta/views"
	"golang.org/x/oauth2"
)

func generateRandomNumber() int {
	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	ulid := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	randomNumber := int(ulid[15])
	return randomNumber

}

func AddPackage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/package" {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodGet {
		templ.Handler(views.AddPackage()).ServeHTTP(w, r)
		return
	}
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		user := r.Context().Value(userContextKey)
		userAuth, ok := user.(*model.User)
		if !ok {
			http.Error(w, "Invalid user context", http.StatusInternalServerError)
			return
		}

		id := generateRandomNumber()
		row := model.Package{
			ID:             id,
			PackageName:    r.FormValue("package_name"),
			PackageVersion: r.FormValue("package_version"),
			PackageURL:     r.FormValue("package_url"),
			UserID:         userAuth.ID,
		}

		var results []model.Package
		err = supabaseClient.DB.From("package").Insert(row).Execute(&results)
		if err != nil {
			fmt.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Println(results)
		http.Redirect(w, r, "/dash", http.StatusFound)
	}
}

type Repository struct {
	Owner     string
	Repo      string
	LatestTag string
	Package   *string
}

func fetchURLsFromDatabase(w http.ResponseWriter, r *http.Request) (map[string]string, error) {
	cookieUserID, err := r.Cookie("user_id")
	if err != nil {
		fmt.Fprintf(w, "Failed to get user_id cookie: %v\n", err)
		return nil, err
	}

	var data []map[string]interface{}
	err = supabaseClient.DB.From("package").Select("package_url", "package_version").Eq("user_id", cookieUserID.Value).Execute(&data)
	if err != nil {
		fmt.Fprintf(w, "Failed to fetch package URLs and versions from the database: %v\n", err)
		return nil, err
	}

	// Construct a map of URLs to their corresponding versions
	urlsAndVersions := make(map[string]string)
	for _, record := range data {
		urlValue, urlOk := record["package_url"].(string)
		versionValue, versionOk := record["package_version"].(string)
		if urlOk && versionOk {
			urlsAndVersions[urlValue] = versionValue
		}
	}

	fmt.Println(urlsAndVersions)
	return urlsAndVersions, nil

}

func parseRepoURL(repoURL string) (string, string, error) {
	parts := strings.Split(strings.TrimPrefix(repoURL, "https://"), "/")
	if len(parts) < 3 || parts[0] != "github.com" {
		return "", "", fmt.Errorf("invalid repository URL format")
	}
	owner, repo := parts[1], parts[2]
	repo = strings.TrimSuffix(repo, "/releases")
	return owner, repo, nil
}

func CheckReleasesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	client := githubClient(ctx)
	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Printf("Error getting rate limits: %v\n", err)
		return
	}

	fmt.Printf("Rate limit: %d/%d\n", rateLimits.Core.Remaining, rateLimits.Core.Limit)

	urlsAndVersions, err := fetchURLsFromDatabase(w, r)
	if err != nil {
		fmt.Fprintf(w, "Failed to fetch URLs from the database: %v\n", err)
		return
	}

	if len(urlsAndVersions) == 0 {
		fmt.Fprintf(w, "No repository URLs found in the database.\n")
		return
	}

	var repos []Repository
	for url, version := range urlsAndVersions {
		owner, repo, err := parseRepoURL(url)
		if err != nil {
			fmt.Printf("Error parsing URL %s: %v\n", url, err)
			continue
		}
		fmt.Println("jjk", owner, repo, version)
		repos = append(repos, Repository{Owner: owner, Repo: repo, LatestTag: version})
	}

	var releaseDataList []model.ReleaseData
	for _, repo := range repos {
		repoReleaseData, err := fetchReleases(ctx, client, repo, repo.LatestTag)

		fmt.Println("jjk", repoReleaseData, err)
		if err != nil {
			fmt.Fprintf(w, "Failed to fetch releases for %s/%s: %v\n", repo.Owner, repo.Repo, err)
			continue
		}
		releaseDataList = append(releaseDataList, repoReleaseData...)
	}

	fmt.Println(releaseDataList)

	// for _, releaseData := range releaseDataList {
		templ.Handler(views.ChartPackage(releaseDataList)).ServeHTTP(w, r)
	// }
}

func fetchReleases(ctx context.Context, client *github.Client, repo Repository, currentTag string) ([]model.ReleaseData, error) {
	releases, _, err := client.Repositories.ListReleases(ctx, repo.Owner, repo.Repo, nil)
	if err != nil {
		return nil, err
	}

	var releaseDataList []model.ReleaseData
	var latestReleaseBody string
	currentVersion, err := parseVersion(currentTag)
	if err != nil {
		return nil, err
	}

	for _, release := range releases {
		releaseTag := release.GetTagName()
		releaseVersion, err := parseVersion(releaseTag)
		if err != nil {
			continue
		}

		if releaseVersion.GreaterThan(currentVersion) {
			releaseData := model.ReleaseData{
				Owner:            repo.Owner,
				Repo:             repo.Repo,
				LatestTag:        releaseTag,
				ReleaseBody:      release.GetBody(),
				CurrentTag:       currentTag,
				ReleaseNotesDiff: compareReleaseNotes(release.GetBody(), latestReleaseBody),
			}
			releaseDataList = append(releaseDataList, releaseData)
			latestReleaseBody = release.GetBody()
		}
	}

	return releaseDataList, nil
}

func parseVersion(tag string) (*semver.Version, error) {
	// Remove any non-numeric characters from the tag
	re := regexp.MustCompile(`[^0-9.]+`)
	numericTag := re.ReplaceAllString(tag, "")

	// Split the numeric part into segments
	segments := strings.Split(numericTag, ".")

	// Create a slice of 64-bit integers from the segments
	versionParts := make([]uint64, len(segments))
	for i, segment := range segments {
		versionParts[i], _ = strconv.ParseUint(segment, 10, 64)
	}

	// Create a semver.Version from the parsed parts
	return semver.NewVersion(fmt.Sprintf("%d.%d.%d", versionParts[0], versionParts[1], versionParts[2]))
}

func githubClient(ctx context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc)
}

// func shouldCheckRelease(tag, latestTag string) bool {
// 	// Remove the leading "v" if present
// 	tag = strings.TrimPrefix(tag, "v")
// 	latestTag = strings.TrimPrefix(latestTag, "v")

// 	// Parse the tag and latestTag as semantic versions
// 	parsedTag, err := version.NewVersion(tag)
// 	if err != nil {
// 		return false
// 	}
// 	parsedLatestTag, err := version.NewVersion(latestTag)
// 	if err != nil {
// 		return false
// 	}

// 	// Check if the parsed tag is newer than the parsed latestTag
// 	return parsedTag.GreaterThan(parsedLatestTag)
// }

// func matchesPackageFilter(release *github.RepositoryRelease, packageFilter *string) bool {
// 	if packageFilter == nil {
// 		return true
// 	}

// 	for _, asset := range release.Assets {
// 		if strings.Contains(asset.GetName(), *packageFilter) {
// 			return true
// 		}
// 	}

// 	return false
// }

func compareReleaseNotes(newNotes, oldNotes string) string {
	if newNotes == oldNotes {
		return "No changes in the release notes."
	}

	newLines := strings.Split(newNotes, "\n")
	oldLines := strings.Split(oldNotes, "\n")

	var differences []string
	for _, newLine := range newLines {
		if !contains(oldLines, newLine) {
			differences = append(differences, newLine)
		}
	}

	if len(differences) == 0 {
		return "No changes in the release notes."
	}

	return "Changes in the release notes:\n" + strings.Join(differences, "\n")
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func ListPackage(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey)
	userAuth, ok := user.(*model.User)
	if !ok {
		http.Error(w, "Invalid user context", http.StatusInternalServerError)
		return
	}

	if userAuth != nil {
		var packages []model.Mypack
		err := supabaseClient.DB.From("package").Select("*").Eq("user_id", userAuth.ID).Execute(&packages)
		if err != nil {
			fmt.Println("No rows returned", err)
		}

		fmt.Println(packages)
		// for _, pack := range packages {
			templ.Handler(views.Dash(packages)).ServeHTTP(w, r)
		// }
	} else {
		fmt.Println("Error: user is not authenticated")
	}
}
