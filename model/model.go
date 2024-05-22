package model

type ReleaseData struct {
	Owner            string
	Repo             string
	LatestTag        string
	ReleaseBody      string
	CurrentTag       string
	ReleaseNotesDiff string
}

type Package struct {
	ID             int    `json:"id"`
	PackageName    string `json:"package_name"`
	PackageVersion string `json:"package_version"`
	PackageURL     string `json:"package_url"`
	UserID         string `json:"user_id"`
}

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
}

type Mypack struct {
	ID             int    `json:"id"`
	PackageName    string `json:"package_name"`
	PackageVersion string `json:"package_version"`
	PackageURL     string `json:"package_url"`
}