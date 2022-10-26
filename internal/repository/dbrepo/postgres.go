package dbrepo

import (
	"database/sql"

	"github.com/DeLuci/coog-music/internal/models"
)

func (m *postgresDBRepo) GetArtists() ([]models.Artist, error) {

	var artists []models.Artist
	// probably need to add a where statement and get rid of *
	query := "SELECT * FROM artists"

	rows, err := m.DB.Query(query)
	if err != nil {
		return nil, err
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	for rows.Next() {
		var artist models.Artist

		rows.Scan(&artist.Name, &artist.Artist_id, &artist.Location, &artist.Join_date, &artist.Songs, &artist.Admin, &artist.Publisher)

		if err != nil {
			return nil, err
		}
		artists = append(artists, artist)
	}
	return artists, nil
}

func (m *postgresDBRepo) AddUser(res models.Users) error {
	query := "insert into users (username, password) values ($1, $2)"

	_, err := m.DB.Exec(query, res.Username, res.Password)
	if err != nil {
		return err
	}

	return nil
}

func (m *postgresDBRepo) AddSong(res models.Song) error {
	query := "insert into song (title, artist_name) values ($1, $2)"

	_, err := m.DB.Exec(query, res.Artist_name, res.Title)
	if err != nil {
		return err
	}

	return nil
}

//TODO: ADD LINKING TABLES AND USE THEM TO GRAB THE OTHER STUFF

func (m *postgresDBRepo) AddSongToPlaylist(song models.Song, playlist models.Playlist) error {
	query := "insert into playlist (playlist.playlist_id, playlist.songs) values($1, $2)"

	_, err := m.DB.Exec(query, playlist.Playlist_id, song.Song_id)
	if err != nil {
		return err
	}

	return nil
}

func (m *postgresDBRepo) PlaySong(res models.Song) error {
	// Do we need this where statement?
	query := "select song_id from song where title == $1"

	_, err := m.DB.Exec(query, res.Song_id)
	if err != nil {
		return err
	}
	//generate a session id for songplay

	return nil
}

func (m *postgresDBRepo) AddSongToAlbum(res models.Song, album models.Album) error {
	query := "select song from song where title == $1"
	add_query := "insert into song(album) values ($1)"

	_, err := m.DB.Exec(query, res.Title)
	if err != nil {
		return err
	}
	_, err2 := m.DB.Exec(add_query, album.Name)
	if err2 != nil {
		return err2
	}

	return nil
}

//play song (select and songplay session)
//add song to album (artist thing
