package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/CoogTunes/coog-music/internal/forms"
	"github.com/CoogTunes/coog-music/internal/render"
	"golang.org/x/crypto/bcrypt"

	"github.com/CoogTunes/coog-music/internal/config"
	"github.com/CoogTunes/coog-music/internal/driver"
	"github.com/CoogTunes/coog-music/internal/models"
	"github.com/go-chi/chi/v5"

	"github.com/CoogTunes/coog-music/internal/repository"
	"github.com/CoogTunes/coog-music/internal/repository/dbrepo"
)

// Repo the repository used by the handlers
var Repo *Repository

var UserCache models.Users

// Repository is the repository type
type Repository struct {
	App *config.AppConfig
	DB  repository.DatabaseRepo
}

// NewRepo creates a new repository
func NewRepo(a *config.AppConfig, db *driver.DB) *Repository {
	return &Repository{
		App: a,
		DB:  dbrepo.NewPostgresRepo(db.SQL, a),
	}
}

// NewHandlers set the repository for the handlers
func NewHandlers(r *Repository) {
	Repo = r
}

// LOGIN/SIGNUP

func (m *Repository) GetLogin(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "login.page.gohtml", &models.TemplateData{
		Form: forms.New(nil),
	})
}

func (m *Repository) PostLogin(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Fatal(err)
	}
	loginOrRegister := r.Form.Get("submit_button")
	fmt.Println(loginOrRegister)
	if loginOrRegister == "register" {
		fmt.Println("Passes to function PostRegistration ")
		m.PostRegistration(w, r)
		return
	}
	//_ = m.App.Session.RenewToken(r.Context())

	email := r.Form.Get("email")
	pwd := r.Form.Get("password")

	userInfo, err := m.DB.Authenticate(email, pwd)
	if err != nil {
		log.Println(err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	UserCache = userInfo
	//m.GetPlaylistsByID(w, r)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (m *Repository) PostRegistration(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Fatal(err)
	}

	pwd := []byte(r.Form.Get("password"))
	hashedPwd, err := bcrypt.GenerateFromPassword(pwd, bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
		return
	}

	adminLevel := r.Form.Get("user-level")
	firstName := r.Form.Get("first_name")
	lastName := r.Form.Get("last_name")
	lvl := 0
	if adminLevel == "user" {
		lvl = 1
	} else if adminLevel == "artist" {
		lvl = 2
	}

	registrationModel := models.Users{
		First_name:  firstName,
		Last_name:   lastName,
		Username:    r.Form.Get("email"),
		Password:    string(hashedPwd),
		Admin_level: lvl,
	}
	users, err := m.DB.AddUser(registrationModel)
	if false {
		log.Println(users)
	}
	if err != nil {
		log.Fatal(err)
	}

	UserCache = users

	if lvl == 2 {
		m.AddArtist(firstName, lastName)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// END LOGIN/SIGNUP--------------------------------------------------------------------------------------

// ADD ARTIST -------------------------------------------------------------------------------------------

func (m *Repository) AddArtist(firstName string, lastName string) {
	artistName := getArtistName(firstName, lastName)

	artistInfo := models.Artist{
		Name:      artistName,
		Artist_id: UserCache.User_id,
		Location:  "no_location",
	}

	err := m.DB.AddArtist(artistInfo)
	if err != nil {
		log.Println("Cannot add artist information")
		return
	}
}

// END ADD ARTIST ----------------------------------------------------------------------------

// HOME PAGE ---------------------------------------------------------------------------------

func (m *Repository) GetHome(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "coogtunes.page.gohtml", &models.TemplateData{
		Form:     forms.New(nil),
		UserData: UserCache,
	})
	m.GetTopSongs(w, r)
	m.GetPlaylistsByID(w, r)
}
func (m *Repository) GetTopSongs(w http.ResponseWriter, r *http.Request) {
	topSongs, err := m.DB.GetTopSongs()
	if err != nil {
		log.Println("Cannot get the top 14 songs")
	}
	returnAsJSON(topSongs, w, err)
}

func (m *Repository) GetPlaylistsByID(w http.ResponseWriter, r *http.Request) {
	playlists, err := m.DB.GetPlaylists(UserCache.User_id)
	if err != nil {
		log.Println("Cannot get the top 14 songs")
	}
	if len(playlists) == 0 {
		emptyPlaylist := models.Playlist{}
		returnAsJSON(emptyPlaylist, w, err)
		return
	}
	returnAsJSON(playlists, w, err)
}

// LOGOUT

func (m *Repository) LogOut(w http.ResponseWriter, r *http.Request) {
	noUserData := models.Users{}
	UserCache = noUserData
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// END OF HOME PAGE ---------------------------------------------------------------------------------

// PROFILE PAGE ---------------------------------------------------------------------------------

func (m *Repository) GetProfile(w http.ResponseWriter, r *http.Request) {
	render.Template(w, r, "profilepage.page.gohtml", &models.TemplateData{
		Form:     forms.New(nil),
		UserData: UserCache,
	})
}

//  END OF PROFILE PAGE ---------------------------------------------------------------------------------

// UPLOAD MUSIC ---------------------------------------------------------------------------------

func (m *Repository) UploadFile(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Passing through upload file handler")
	err := r.ParseMultipartForm(32 << 20) // 32mb
	if err != nil {
		log.Fatal("Cannot parse upload Files")
	}

	songOrAlbum := r.Form.Get("uploadType")
	if err != nil {
		fmt.Println("cannot parse the image file")
	}

	if songOrAlbum == "song" {
		fmt.Println("Passing through the upload song handler")
		m.UploadSong(w, r)
		return
	}

	m.UploadAlbum(w, r)
}

func (m *Repository) UploadSong(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		log.Fatal("Could not parse multipart forms")
	}

	artistName := concatenateName(r.Form.Get("artist_name"))
	songName := r.Form.Get("music_name")
	fmt.Println("Passes through the songName")
	coverFile, fhCover, err := r.FormFile("music_cover")
	if err != nil {
		log.Println("Cannot read cover_file")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println("Passes through coverFile")

	coverPath := "./static/media/artist/" + artistName
	err = os.MkdirAll(coverPath, os.ModePerm)
	if os.IsExist(err) {
		log.Println("Cover jpeg is already in directory")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dst, err := os.Create(fmt.Sprintf(coverPath+"/"+"%s", fhCover.Filename))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(dst, coverFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fullCoverPath := coverPath + "/" + fhCover.Filename
	coverFile.Close()
	dst.Close()

	songFile, fhSong, err := r.FormFile("music_audio")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	songPath := "./static/media/artist/" + artistName
	err = os.MkdirAll(songPath, os.ModePerm)
	if os.IsExist(err) {
		log.Println("Song audio is already in directory")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dst2, err := os.Create(fmt.Sprintf(songPath+"/"+"%s", fhSong.Filename))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(dst2, songFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fullSongPath := songPath + "/" + fhSong.Filename
	coverFile.Close()
	dst.Close()

	songInfo := models.Song{
		Title:     songName,
		Artist_id: UserCache.User_id,
		CoverPath: fullCoverPath,
		SongPath:  fullSongPath,
	}

	err = m.DB.AddSong(songInfo)
	if err != nil {
		log.Println("Cannot add song to the database")
	}

	fmt.Fprintf(w, "Upload successful")

}

func (m *Repository) UploadAlbum(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		log.Fatal("Could not parse multipart forms")
	}
	artistName := concatenateName(r.Form.Get("artist_name"))
	albumName := r.Form.Get("music_name")
	pathAlbumName := concatenateName(albumName)
	coverFile, fhCover, err := r.FormFile("music_cover")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	coverPath := "./static/media/artist/" + artistName + "/" + pathAlbumName
	err = os.MkdirAll(coverPath, os.ModePerm)
	if os.IsExist(err) {
		log.Println("Cover jpeg is already in directory")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dst, err := os.Create(fmt.Sprintf(coverPath+"/"+"%s", fhCover.Filename))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(dst, coverFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fullCoverPath := coverPath + "/" + fhCover.Filename
	coverFile.Close()
	dst.Close()

	albumInfo := models.Album{
		Name:      albumName,
		Artist_id: UserCache.User_id,
	}

	albumDBInfo, err := m.DB.AddAlbum(albumInfo)

	if err != nil {
		log.Println("Cannot add album")
		return
	}
	files := r.MultipartForm.File["music_audio"]
	for _, fileHeader := range files {
		fileSize := fileHeader.Size
		fmt.Println(fileHeader.Filename)
		file, err := fileHeader.Open()
		if err != nil {
			fmt.Println("Could not open the file")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		buff := make([]byte, fileSize)
		_, err = file.Read(buff)
		if err != nil {
			fmt.Println("Could not read the file")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		filetype := http.DetectContentType(buff)
		if filetype != "image/jpeg" && filetype != "audio/mpeg" {
			http.Error(w, "The provided file format is not allowed. Please upload a JPEG image or MP3 file", http.StatusBadRequest)
			return
		}

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		filePath := "./static/media/artist/" + artistName + "/" + pathAlbumName
		err = os.MkdirAll(filePath, os.ModePerm)
		if os.IsExist(err) {
			fmt.Println("Song already exists!")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		f, err := os.Create(fmt.Sprintf(filePath+"/"+"%s", fileHeader.Filename))
		if err != nil {
			fmt.Println("Could not create the named file")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()
		_, err = io.Copy(f, file)
		if err != nil {
			fmt.Println("Could not copy the uploaded files")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		title := strings.ReplaceAll(fileHeader.Filename, filepath.Ext(fileHeader.Filename), "")
		newTitle := splitName(title)
		songPath := filePath + "/" + fileHeader.Filename
		songInfo := models.Song{
			Title:     newTitle,
			Album:     albumName,
			SongPath:  songPath,
			CoverPath: fullCoverPath,
			Artist_id: UserCache.User_id,
			Album_id:  albumDBInfo.Album_id,
		}
		err = m.DB.AddSongForAlbum(songInfo)
		if err != nil {
			log.Println("Cannot add song")
			return
		}
	}

}

// END OF UPLOAD MUSIC ---------------------------------------------------------------------------------

// SEARCH SECTION --------------------------------------------------------------------------------------
func (m *Repository) Search(w http.ResponseWriter, r *http.Request) (string, string) {
	target := r.URL.Query().Get("strTarget")
	filter := r.URL.Query().Get("filters")
	decodedValue, err := url.QueryUnescape(target)
	if err != nil {
		log.Print("Could not decode the parameter")
	}
	return decodedValue, filter
}

// PLAYLIST SECTION ---------------------------------------------------------------------------------
// TODO: Return empty json when getting an error on
func (m *Repository) PlaylistSearch(w http.ResponseWriter, r *http.Request) {
	decodedValue, filter := m.Search(w, r)
	if filter == "song" {
		songInfo, err := m.DB.GetSongsByName(decodedValue)
		if err != nil {
			returnAsJSON(songInfo, w, err)
			log.Println("Cannot get songs!")
		}
		returnAsJSON(songInfo, w, err)
		return
	} else if filter == "album" {
		albumInfo, err := m.DB.GetSongsFromAlbum(decodedValue)
		if err != nil {
			log.Println("Cannot get songs!")
		}
		returnAsJSON(albumInfo, w, err)
		return
	} else {

	}
}

type PlayListJson struct {
	PlayListName string   `json:"playlistName"`
	PlayList     []string `json:"playListItems"`
}

func (m *Repository) InsertPlaylist(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var playlistInfo PlayListJson
	err := decoder.Decode(&playlistInfo)

	log.Println(playlistInfo.PlayListName)

	if err != nil {
		log.Println("Cannot decode the json")
	}
	playlist := models.Playlist{
		User_id: UserCache.User_id,
		Name:    playlistInfo.PlayListName,
	}

	plylist, err := m.DB.AddPlaylist(playlist)
	if err != nil {
		log.Println("Cannot add playlist")
		return
	}
	for _, strNum := range playlistInfo.PlayList {
		songID, err := strconv.Atoi(strNum)
		if err != nil {
			log.Println("Cannot convert string to num")
		}
		err = m.DB.AddPlaylistSong(songID, plylist.Playlist_id)
		if err != nil {
			log.Println("Cannot add playlist")
		}
	}

	returnAsJSON(plylist, w, err)

}

type GetSongsFromPlaylist struct {
	PlayListName string `json:"playlistName"`
	PlayListID   string `json:"playlistID"`
}

func (m *Repository) GetPlaylistSongs(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var playlistInfo GetSongsFromPlaylist
	err := decoder.Decode(&playlistInfo)
	if err != nil {
		log.Println("Cannot decode the json")
	}
	playListIO, err := strconv.Atoi(playlistInfo.PlayListID)
	if err != nil {
		log.Println("Cannot convert string to int")
	}
	songsFromPlaylistID := models.Playlist{
		Playlist_id: playListIO,
	}

	m.DB.

}

// END PLAYLIST SECTION --------------------------------------------------------------------------------

// USERS ------------------------------------------------------------------------

func (m *Repository) AddUser(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	// get fields
	username := r.Form.Get("username")
	password := r.Form.Get("password")
	first_name := r.Form.Get("first_name")
	last_name := r.Form.Get("last_name")
	stringAdmin := r.Form.Get("admin")

	admin, err := strconv.Atoi(stringAdmin)
	if err != nil {
		log.Println(err)
	}

	newUser := models.Users{Username: username, Password: password, First_name: first_name, Last_name: last_name, Admin_level: admin}

	addedUser, err := m.DB.AddUser(newUser)

	returnAsJSON(addedUser, w, err)
}

func (m *Repository) GetUser(w http.ResponseWriter, r *http.Request) {

	id := chi.URLParam(r, "id")

	user, err := m.DB.GetUser(id)

	returnAsJSON(user, w, err)
}

// END USERS -------------------------------------------------------------------

// ARTISTS
//
//	func (m *Repository) AddArtist(w http.ResponseWriter, r *http.Request) {
//		r.ParseForm()
//		// get fields
//		name := r.Form.Get("name")
//		artist_id := r.Form.Get("artist_id")
//		location := r.Form.Get("location")
//		join_date := r.Form.Get("join_date")
//		var songs []models.Song
//
//		int_artist_id, err := strconv.Atoi(artist_id)
//		if err != nil {
//			log.Println(err)
//		}
//		// joindate and songs[] should be empty to start.
//		artistToAdd := models.Artist{Name: name, Artist_id: int_artist_id, Location: location, Join_date: join_date, Songs: songs}
//
//		addedArtist, err := m.DB.AddArtist(artistToAdd)
//
//		returnAsJSON(addedArtist, w, err)
//
// }
// ARTISTS
//
//	func (m *Repository) AddArtist(w http.ResponseWriter, r *http.Request) {
//		r.ParseForm()
//		// get fields
//		name := r.Form.Get("name")
//		artist_id := r.Form.Get("artist_id")
//		location := r.Form.Get("location")
//		join_date := r.Form.Get("join_date")
//		var songs []models.Song
//
//		int_artist_id, err := strconv.Atoi(artist_id)
//		if err != nil {
//			log.Println(err)
//		}
//		// joindate and songs[] should be empty to start.
//		artistToAdd := models.Artist{Name: name, Artist_id: int_artist_id, Location: location, Join_date: join_date, Songs: songs}
//
//		addedArtist, err := m.DB.AddArtist(artistToAdd)
//
//		returnAsJSON(addedArtist, w, err)
//
// }
func (m *Repository) GetArtists(w http.ResponseWriter, r *http.Request) {
	artists, err := m.DB.GetArtists()

	returnAsJSON(artists, w, err)
}

//
//func (m *Repository) GetArtistsAndSongs(w http.ResponseWriter, r *http.Request) {
//	artists, err := m.DB.GetArtistsAndSongs()
//	returnAsJSON(artists, w, err)
//}

func (m *Repository) UpdateArtist(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	// get fields
	name := r.Form.Get("name")
	// artist_idString := r.Form.Get("artist_id")
	location := r.Form.Get("location")
	// join_date := r.Form.Get("join_date")

	// artist_id, err := strconv.Atoi(artist_idString)
	// if err != nil {
	// 	log.Println(err)
	// }

	artistToUpdate := models.Artist{Name: name, Location: location}

	addedUser, err := m.DB.UpdateArtist(artistToUpdate)

	returnAsJSON(addedUser, w, err)
}

// ALBUMS

//func (m *Repository) AddAlbum(w http.ResponseWriter, r *http.Request) {
//	var album models.Album
//	var err error
//	r.ParseForm()
//	// get fields
//	album.Artist_id, err = strconv.Atoi(r.Form.Get("artist_id"))
//	if err != nil {
//		log.Println(err)
//	}
//	album.Name = r.Form.Get("album_name")
//	album.Date_added = r.Form.Get("album_date")
//	addedAlbum, err := m.DB.AddAlbum(album)
//
//	returnAsJSON(addedAlbum, w, err)
//}

// SONGS

//func (m *Repository) AddSong(w http.ResponseWriter, r *http.Request) {
//	r.ParseForm()
//	// get fields
//	title := r.Form.Get("title")
//	release_date := r.Form.Get("release_date")
//	duration := r.Form.Get("duration")
//	artist_id_string := r.Form.Get("artist_id")
//	album_id_string := r.Form.Get("album_id")
//
//	artist_id, err := strconv.Atoi(artist_id_string)
//	if err != nil {
//
//	}
//	album_id, err := strconv.Atoi(album_id_string)
//	if err != nil {
//
//	}
//	songToAdd := models.Song{
//		Title:        title,
//		Release_date: release_date,
//		Duration:     duration,
//		Album_id:     album_id,
//		Artist_id:    artist_id,
//	}
//
//	addedSong, err := m.DB.AddSong(songToAdd)
//	if err != nil {
//		log.Println(err)
//	}
//
//	addedSong.Artist_name, err = m.DB.GetArtistName(artist_id)
//	if err != nil {
//		log.Println(err)
//	}
//	returnAsJSON(addedSong, w, err)
//}

//func (m *Repository) GetSongs(w http.ResponseWriter, r *http.Request) {
//	songs, err := m.DB.GetSongs()
//
//	returnAsJSON(songs, w, err)
//}
//

//func (m *Repository) GetSong(w http.ResponseWriter, r *http.Request) {
//	x := chi.URLParam(r, "id")
//	song, err := m.DB.GetSong(x)
//	returnAsJSON(song, w, err)
//}

//func (m *Repository) UpdateSong(w http.ResponseWriter, r *http.Request) {
//	r.ParseForm()
//	// get fields
//	title := r.Form.Get("title")
//	duration := r.Form.Get("duration")
//	song_id_string := r.Form.Get("song_id")
//
//	song_id, err := strconv.Atoi(song_id_string)
//	if err != nil {
//		log.Println(err)
//	}
//
//	songToUpdate := models.Song{
//		Title:    title,
//		Duration: duration,
//		Song_id:  song_id,
//	}
//
//	updatedSong, err := m.DB.UpdateSong(songToUpdate)
//	if err != nil {
//		log.Println(err)
//	}
//
//	returnAsJSON(updatedSong, w, err)
//}

//func (m *Repository) UpdateSongCount(w http.ResponseWriter, r *http.Request) {
//	idString := chi.URLParam(r, "id")
//	id, err := strconv.Atoi(idString)
//	if err != nil {
//		log.Println(err)
//	}
//	var songWithId models.Song = models.Song{Song_id: id}
//	song, err := m.DB.UpdateSongCount(songWithId)
//	returnAsJSON(song, w, err)
//}

func (m *Repository) AddSongToPlaylist(w http.ResponseWriter, r *http.Request) {
	return
}

func (m *Repository) AddSongToAlbum(w http.ResponseWriter, r *http.Request) {
	return
}

func (m *Repository) GetPlaylists(w http.ResponseWriter, r *http.Request) {
	//playlists, err := m.DB.GetPlaylists()
	//
	//returnAsJSON(playlists, w, err)
	return
}

func (m *Repository) GetAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := m.DB.GetAlbums()

	returnAsJSON(albums, w, err)
}

func (m *Repository) UpdateUser(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	// get fields
	userIdString := r.Form.Get("user_id")
	username := r.Form.Get("username")
	password := r.Form.Get("password")
	first_name := r.Form.Get("first_name")
	last_name := r.Form.Get("last_name")
	stringAdmin := r.Form.Get("admin_level")

	userId, err := strconv.Atoi(userIdString)
	if err != nil {
		log.Println(err)
	}

	admin, err := strconv.Atoi(stringAdmin)
	if err != nil {
		log.Println(err)
	}

	newUser := models.Users{User_id: userId, Username: username, Password: password, First_name: first_name, Last_name: last_name, Admin_level: admin}

	addedUser, err := m.DB.UpdateUser(newUser)

	returnAsJSON(addedUser, w, err)
}

func (m *Repository) Follow(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	// get fields
	artist_idString := r.Form.Get("artist_id")
	user_idString := r.Form.Get("user_id")

	artistId, err := strconv.Atoi(artist_idString)
	if err != nil {
		log.Println(err)
	}
	userId, err := strconv.Atoi(user_idString)
	if err != nil {
		log.Println(err)
	}

	follower, err := m.DB.Follow(artistId, userId)
	returnAsJSON(follower, w, err)
}

func (m *Repository) AddOrUpdateLikeValue(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	// get fields

	isLike, err := strconv.ParseBool(r.Form.Get("is_like"))
	if err != nil {
		log.Println(err)
	}
	song_id, err := strconv.Atoi(r.Form.Get("song_id"))
	if err != nil {
		log.Println(err)
	}
	user_id, err := strconv.Atoi(r.Form.Get("user_id"))
	if err != nil {
		log.Println(err)
	}
	err2 := m.DB.AddOrUpdateLikeValue(isLike, song_id, user_id)
	if err2 != nil {
		log.Println(err)
	}
}

// REPORTS
func (m *Repository) GetLikesReport(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	// get fields

	minLikes, err := strconv.Atoi(r.Form.Get("min_likes"))
	if err != nil {
		log.Println(err)
	}
	maxLikes, err := strconv.Atoi(r.Form.Get("max_likes"))
	if err != nil {
		log.Println(err)
	}
	minDislikes, err := strconv.Atoi(r.Form.Get("min_dislikes"))
	if err != nil {
		log.Println(err)
	}
	maxDislikes, err := strconv.Atoi(r.Form.Get("max_dislikes"))
	if err != nil {
		log.Println(err)
	}

	likesReport, err := m.DB.GetLikesReport(minLikes, maxLikes, minDislikes, maxDislikes)
	returnAsJSON(likesReport, w, err)
}

func (m *Repository) GetUserOrArtistReport(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	// get fields
	userType := r.Form.Get("user_type")
	minDate := r.Form.Get("min_date")
	maxDate := r.Form.Get("max_date")
	if userType == "User" || userType == "user" {
		usersReport, err := m.DB.GetUsersReport(minDate, maxDate)
		returnAsJSON(usersReport, w, err)
	} else if userType == "Artist" || userType == "artist" {
		artistReport, err := m.DB.GetArtistReport(minDate, maxDate)
		returnAsJSON(artistReport, w, err)
	}
}

func (m *Repository) GetSongReport(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	// get fields

	min_plays, err := strconv.Atoi(r.Form.Get("min_plays"))
	if err != nil {
		log.Println(err)
	}
	max_plays, err := strconv.Atoi(r.Form.Get("max_plays"))
	if err != nil {
		log.Println(err)
	}
	minDate := r.Form.Get("min_date")
	maxDate := r.Form.Get("max_date")

	songReport, err := m.DB.GetSongReport(minDate, maxDate, min_plays, max_plays)
	returnAsJSON(songReport, w, err)

}

// HELPER FUNCTIONS
// i is the models.XYZ property
func returnAsJSON(i interface{}, w http.ResponseWriter, err error) {
	if err != nil {
		log.Println(err)
	}
	j, _ := json.MarshalIndent(i, "", "   ")
	log.Println(string(j))
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(j)
	if err != nil {
		log.Print(err)
	}
}

func returnTopSongsJSON(songs []models.Song, w http.ResponseWriter, err error) {
	if err != nil {
		log.Println(err)
	}

	for _, song := range songs {
		j, _ := json.MarshalIndent(song, "", "   ")
		log.Println(string(j))
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(j)
		if err != nil {
			log.Print(err)
		}
	}

}

func getArtistName(firstName string, lastName string) string {
	artistName := ""
	if lastName != "" {
		artistName = firstName + " " + lastName
	} else {
		artistName = firstName
	}
	return artistName
}

func concatenateName(artistName string) string {
	splitString := strings.Split(artistName, " ")
	newString := strings.Join(splitString, "_")
	return newString
}

func splitName(titleName string) string {
	splitString := strings.Split(titleName, "_")
	newString := strings.Join(splitString, " ")
	return newString
}
