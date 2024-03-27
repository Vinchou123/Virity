package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Fonction principale
func main() {
	// Créer une instance d'Echo
	e := echo.New()

	// Utiliser le middleware pour les sessions
	e.Use(session.Middleware(sessions.NewCookieStore([]byte("secret"))))

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Connexion à MySQL
	var err error
	db, err = sql.Open("mysql", "ADMIN:CLE@tcp(0.0.0.0:3306)/CoffreFortDb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// Vérifier si l'utilisateur admin existe déjà
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "admin").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	// Si l'utilisateur admin n'existe pas, le créer
	if count == 0 {
		// Hacher le mot de passe avec bcrypt
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("cle"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatal(err)
		}

		// Exécuter la requête d'insertion pour créer un nouvel utilisateur admin avec le mot de passe haché
		_, err = db.Exec("INSERT INTO users (username, password, role) VALUES (?, ?, ?)", "admin", hashedPassword, "admin")
		if err != nil {
			log.Fatal(err)
		}
	}

	// Test de la connexion à MySQL
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MySQL!")

	// Routes
	e.GET("/", homeHandler)
	e.GET("/users", listUsersHandler)   // Afficher tous les utilisateurs
	e.GET("/register", registerHandler) // Page d'inscription (affichage du formulaire)
	e.POST("/register", registerPostHandler)
	e.GET("/delete", deleteFormHandler) // Afficher le formulaire de suppression
	e.POST("/delete", deleteHandler)    // Supprimer un utilisateur
	e.GET("/login", loginHandler)       // Page de connexion
	e.POST("/login", loginPostHandler)  // Traitement du formulaire de connexion
	e.POST("/logout", logoutHandler)    // Déconnexion de l'utilisateur
	e.GET("/welcome", welcomeHandler)
	e.GET("/create-note", createNoteHandler)      // Afficher le formulaire pour créer une note
	e.POST("/create-note", createNotePostHandler) // Traitement du formulaire pour créer une note
	e.GET("/upload-file", uploadFileHandler)      // Afficher le formulaire pour déposer un fichier
	e.POST("/upload-file", uploadFilePostHandler) // Traitement du formulaire pour déposer un fichier
	// Route pour visualiser le contenu du fichier
	e.GET("/view-file/:fileName", viewFileHandler)
	e.DELETE("/files/:id", deleteFileHandler)
	// Route pour supprimer un fichier
	e.POST("/delete-file/:fileName", deleteFileHandler)
	// Route pour la suppression du compte utilisateur
	e.POST("/delete-account", deleteAccountHandler)
	// Route pour afficher la page de confirmation de suppression de compte
	e.GET("/goodbye", func(c echo.Context) error {
		return c.File("goodbye.html")
	})
	// Ajoutez cette ligne dans votre fonction main() pour configurer la gestion de la route DELETE
	e.DELETE("/delete-account", deleteAccountHandler)

	e.POST("/delete-note/:id", deleteNoteHandler)
	e.POST("/upload-file", uploadFileHandler)

	// Démarrage du serveur
	e.Start(":8081")
}

// Fonction pour gérer la page d'accueil et la page d'accueil de l'administrateur
func welcomeHandler(c echo.Context) error {
    sess, _ := session.Get("session", c)
    username, ok := sess.Values["username"].(string)
    if !ok {
        return c.Redirect(http.StatusSeeOther, "/login")
    }

    if username == "admin" {
        htmlContent, err := ioutil.ReadFile("accueilAdmin.html")
        if err != nil {
            return err
        }
        return c.HTML(http.StatusOK, string(htmlContent))
    }

    userID, ok := sess.Values["userID"].(int)
    if !ok {
        return c.Redirect(http.StatusSeeOther, "/login")
    }

    rows, err := db.Query("SELECT id, title, content FROM notes WHERE user_id = ?", userID)
    if err != nil {
        log.Println("Erreur lors de la récupération des notes :", err)
        return err
    }
    defer rows.Close()

    fileRows, err := db.Query("SELECT filename FROM files WHERE user_id = ?", userID)
    if err != nil {
        log.Println("Erreur lors de la récupération des fichiers de l'utilisateur :", err)
        return err
    }
    defer fileRows.Close()

    type Note struct {
        ID      int
        Title   string
        Content string
    }

    var notes []Note
    for rows.Next() {
        var note Note
        err := rows.Scan(&note.ID, &note.Title, &note.Content)
        if err != nil {
            log.Println("Erreur lors de la lecture des résultats de la requête des notes :", err)
            return err
        }
        notes = append(notes, note)
    }

    var files []string
    for fileRows.Next() {
        var fileName string
        err := fileRows.Scan(&fileName)
        if err != nil {
            log.Println("Erreur lors de la lecture des résultats de la requête des fichiers :", err)
            return err
        }
        files = append(files, fileName)
    }

    var notesHTML string
    for _, note := range notes {
        notesHTML += "<div>"
        notesHTML += "<span><strong>" + note.Title + "</strong><br>" + note.Content + "</span>"
        notesHTML += `<button class="bin-button" onclick="deleteNote(` + strconv.Itoa(note.ID) + `)">
        <svg class="bin-top" viewBox="0 0 39 7" fill="none" xmlns="http://www.w3.org/2000/svg">
            <line y1="5" x2="39" y2="5" stroke="white" stroke-width="4"></line>
            <line x1="12" y1="1.5" x2="26.0357" y2="1.5" stroke="white" stroke-width="3"></line>
        </svg>
        <svg class="bin-bottom" viewBox="0 0 33 39" fill="none" xmlns="http://www.w3.org/2000/svg">
            <mask id="path-1-inside-1_8_19" fill="white">
                <path d="M0 0H33V35C33 37.2091 31.2091 39 29 39H4C1.79086 39 0 37.2091 0 35V0Z"></path>
            </mask>
            <path d="M0 0H33H0ZM37 35C37 39.4183 33.4183 43 29 43H4C-0.418278 43 -4 39.4183 -4 35H4H29H37ZM4 43C-0.418278 43 -4 39.4183 -4 35V0H4V35V43ZM37 0V35C37 39.4183 33.4183 43 29 43V35V0H37Z" fill="white" mask="url(#path-1-inside-1_8_19)"></path>
            <path d="M12 6L12 29" stroke="white" stroke-width="4"></path>
            <path d="M21 6V29" stroke="white" stroke-width="4"></path>
        </svg>
    </button>`
        notesHTML += "</div>"
    }

    var filesHTML string
    for _, fileName := range files {
        filesHTML += `<div>
        <span>` + fileName + `</span>
        <button onclick="deleteFile('` + fileName + `')">Supprimer</button>
        <button class="open-file" onclick="window.open('/view-file/` + fileName + `', '_blank')">
            <span class="file-wrapper">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 71 67">
                    <path stroke-width="5" stroke="black" d="M41.7322 11.7678L42.4645 12.5H43.5H68.5V64.5H2.5V2.5H32.4645L41.7322 11.7678Z"></path>
                </svg>
                <span class="file-front"></span>
            </span>
            Open file
        </button>
    </div>`
    }

    uploadForm := `
        <h2>Télécharger un fichier :</h2>
        <form action="/upload-file" method="post" enctype="multipart/form-data">
            <input type="file" name="file" required><br>
            <button type="submit">Télécharger</button>
        </form>
    `

    htmlContent, err := ioutil.ReadFile("welcome.html")
    if err != nil {
        return err
    }

    responseHTML := fmt.Sprintf(string(htmlContent), username, uploadForm, notesHTML, filesHTML)

     // Renvoyer la réponse HTML complète
	 return c.HTML(http.StatusOK, responseHTML)
}

func homeHandler(c echo.Context) error {
	// Lecture du fichier HTML
	tmpl, err := template.ParseFiles("home.html")
	if err != nil {
		// Gérer l'erreur
		return err
	}

	// Exécution du modèle et écriture de la réponse
	err = tmpl.Execute(c.Response().Writer, nil)
	if err != nil {
		// Gérer l'erreur
		return err
	}

	return nil
}

// Gestionnaire de route pour créer une note
func createNotePostHandler(c echo.Context) error {
	// Récupérer le titre et le contenu de la note à partir du formulaire
	title := c.FormValue("title")
	content := c.FormValue("content")

	// Récupérer l'ID de l'utilisateur à partir de la session
	sess, err := session.Get("session", c)
	if err != nil {
		log.Println("Erreur lors de la récupération de la session :", err)
		return err
	}
	userID, ok := sess.Values["userID"].(int)
	if !ok {
		log.Println("ID de l'utilisateur introuvable dans la session")
		return err
	}

	// Insérer la note dans la base de données avec l'ID de l'utilisateur
	_, err = db.Exec("INSERT INTO notes (user_id, title, content) VALUES (?, ?, ?)", userID, title, content)
	if err != nil {
		log.Println("Erreur lors de l'insertion de la note :", err)
		return err
	}

	// Construire une structure de réponse JSON contenant les détails de la note créée
	response := struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}{
		Title:   title,
		Content: content,
	}

	// Retourner la réponse JSON
	return c.JSON(http.StatusOK, response)
}

func createNoteHandler(c echo.Context) error {
	// Afficher un formulaire pour créer une note
	htmlContent := `
        <h1>Créer une note</h1>
        <form action="/create-note" method="post">
            <input type="text" name="title" placeholder="Titre de la note" required><br>
            <textarea name="content" rows="5" cols="50" placeholder="Contenu de la note" required></textarea><br>
            <button type="submit">Créer la note</button>
        </form>
        <a href="/welcome">Retour</a>
    `
	return c.HTML(http.StatusOK, htmlContent)
}
func deleteNoteHandler(c echo.Context) error {
	noteID := c.Param("id")

	// Supprimer la note correspondante dans la base de données
	_, err := db.Exec("DELETE FROM notes WHERE id = ?", noteID)
	if err != nil {
		log.Println("Erreur lors de la suppression de la note :", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Erreur lors de la suppression de la note"})
	}

	// Répondre avec un code de succès
	return c.JSON(http.StatusOK, map[string]string{"message": "Note supprimée avec succès"})
}

// Fonction pour gérer le téléchargement de fichiers
func uploadFilePostHandler(c echo.Context) error {
	// Vérifier si le dossier "uploads" existe, sinon le créer
	if _, err := os.Stat("uploads"); os.IsNotExist(err) {
		err := os.Mkdir("uploads", 0755)
		if err != nil {
			log.Println("Erreur lors de la création du dossier 'uploads' :", err)
			return err
		}
	}

	// Récupérer le fichier depuis le formulaire
	file, err := c.FormFile("file")
	if err != nil {
		log.Println("Erreur lors de la récupération du fichier :", err)
		return err
	}

	// Récupérer l'ID de l'utilisateur à partir de la session
	sess, _ := session.Get("session", c)
	userID := sess.Values["userID"].(int)

	// Enregistrer le fichier sur le serveur
	err = SaveFileToFileSystem(file, userID)
	if err != nil {
		log.Println("Erreur lors de l'enregistrement du fichier sur le système de fichiers :", err)
		return err
	}

	// Enregistrer les détails du fichier dans la base de données
	uploadedFile := UploadedFile{
		UserID:     userID,
		FileName:   file.Filename,
		FilePath:   "uploads/" + file.Filename,
		UploadedAt: time.Now(),
	}

	if err := saveUploadedFileToDatabase(uploadedFile); err != nil {
		log.Println("Erreur lors de l'enregistrement du fichier dans la base de données :", err)
		return err
	}

	// Rediriger vers la page de bienvenue après avoir déposé le fichier
	return c.Redirect(http.StatusSeeOther, "/welcome")
}

// Fonction pour enregistrer le fichier sur le système de fichiers
func SaveFileToFileSystem(file *multipart.FileHeader, userID int) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// Encodage du nom de fichier
	encodedFileName := url.PathEscape(file.Filename)

	// Créer un répertoire pour chaque utilisateur s'il n'existe pas déjà
	userDir := fmt.Sprintf("uploads/%d", userID)
	if _, err := os.Stat(userDir); os.IsNotExist(err) {
		err := os.Mkdir(userDir, 0755)
		if err != nil {
			return err
		}
	}

	// Créer un fichier de destination dans le répertoire de l'utilisateur
	dst, err := os.Create(filepath.Join(userDir, encodedFileName))
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copier le contenu du fichier téléchargé dans le fichier de destination
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	return nil
}

// Définir une structure pour représenter un fichier téléchargé
type UploadedFile struct {
	UserID     int       // ID de l'utilisateur qui a téléchargé le fichier
	FileName   string    // Nom du fichier
	FilePath   string    // Chemin d'accès complet du fichier sur le serveur
	UploadedAt time.Time // Date et heure du téléchargement
}

// Fonction pour récupérer l'ID de l'utilisateur à partir de la session
func getUserIDFromSession(c echo.Context) (int, error) {
	// Récupérer la session à partir du contexte Echo
	sess, err := session.Get("session", c)
	if err != nil {
		return 0, err
	}

	// Vérifier si la session contient la clé userID
	userID, ok := sess.Values["userID"].(int)
	if !ok {
		// Si la clé userID n'est pas présente dans la session, retourner une erreur
		return 0, errors.New("ID de l'utilisateur non trouvé dans la session")
	}

	// Retourner l'ID de l'utilisateur
	return userID, nil
}

// Fonction pour gérer le téléchargement de fichiers
func uploadFileHandler(c echo.Context) error {
	if _, err := os.Stat("uploads"); os.IsNotExist(err) {
		err := os.Mkdir("uploads", 0755)
		if err != nil {
			log.Println("Erreur lors de la création du dossier 'uploads' :", err)
			return err
		}
	}
	// Gérer le fichier uploadé
	file, err := c.FormFile("file")
	if err != nil {
		log.Println("Erreur lors du téléchargement du fichier :", err)
		return err
	}

	// Sauvegarder le fichier sur le serveur
	src, err := file.Open()
	if err != nil {
		log.Println("Erreur lors de l'ouverture du fichier uploadé :", err)
		return err
	}
	defer src.Close()

	// Créer un fichier de destination
	dst, err := os.Create("uploads/" + file.Filename)
	if err != nil {
		log.Println("Erreur lors de la création du fichier de destination :", err)
		return err
	}
	defer dst.Close()

	// Copier le contenu du fichier uploadé dans le fichier de destination
	if _, err = io.Copy(dst, src); err != nil {
		log.Println("Erreur lors de la copie du contenu du fichier :", err)
		return err
	}

	// Enregistrement du fichier dans la base de données
	userID, err := getUserIDFromSession(c)
	if err != nil {
		log.Println("Erreur lors de la récupération de l'ID de l'utilisateur :", err)
		return err
	}

	uploadedFile := UploadedFile{
		UserID:     userID,
		FileName:   file.Filename,
		FilePath:   "uploads/" + file.Filename,
		UploadedAt: time.Now(),
	}

	if err := saveUploadedFileToDatabase(uploadedFile); err != nil {
		log.Println("Erreur lors de l'enregistrement du fichier dans la base de données :", err)
		return err
	}

	// Rediriger l'utilisateur vers la page d'accueil après le téléchargement du fichier
	return c.Redirect(http.StatusSeeOther, "/welcome")
}

// Fonction pour enregistrer le fichier téléchargé dans la base de données
func saveUploadedFileToDatabase(file UploadedFile) error {
	// Ouvrir une connexion à la base de données
	db, err := sql.Open("mysql", "ADMIN:CLE@tcp(192.168.252.2:3306)/CoffreFortDb")
	if err != nil {
		return err
	}
	defer db.Close()

	// Préparer la requête d'insertion
	stmt, err := db.Prepare("INSERT INTO files (user_id, filename, file_path, uploaded_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Exécuter la requête d'insertion avec les valeurs du fichier téléchargé
	_, err = stmt.Exec(file.UserID, file.FileName, file.FilePath, file.UploadedAt)
	if err != nil {
		return err
	}

	return nil
}

// Page de connexion (affichage du formulaire)
func loginHandler(c echo.Context) error {
	// Lecture du fichier HTML
	tmpl, err := template.ParseFiles("login.html")
	if err != nil {
		// Gérer l'erreur
		return err
	}

	// Exécution du modèle et écriture de la réponse
	err = tmpl.Execute(c.Response().Writer, nil)
	if err != nil {
		// Gérer l'erreur
		return err
	}

	return nil
}

// Traitement du formulaire de connexion
func loginPostHandler(c echo.Context) error {
	// Récupérer le nom d'utilisateur et le mot de passe à partir du formulaire
	username := c.FormValue("username")
	password := c.FormValue("password")

	// Vérifier si les informations d'identification sont correctes en comparant avec celles stockées dans la base de données
	var storedPassword string
	var userID int
	err := db.QueryRow("SELECT id, password FROM users WHERE username = ?", username).Scan(&userID, &storedPassword)
	if err != nil {
		// Gérer le cas où l'utilisateur n'existe pas
		log.Println("Utilisateur non trouvé :", err)
		return c.File("userNoFind.html")
	}

	// Vérifier si le mot de passe correspond
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
	if err != nil {
		// Gérer le cas où le mot de passe est incorrect
		log.Println("Mot de passe incorrect :", err)
		return c.File("userNoFind.html")
	}

	// Stocker l'ID de l'utilisateur dans la session
	sess, err := session.Get("session", c)
	if err != nil {
		log.Println("Erreur lors de la récupération de la session :", err)
		return err
	}
	sess.Values["userID"] = userID
	sess.Values["username"] = username
	sess.Save(c.Request(), c.Response())

	// Redirection vers la page de bienvenue
	return c.Redirect(http.StatusSeeOther, "/welcome")
}

// Page pour afficher tous les utilisateurs existants
func listUsersHandler(c echo.Context) error {
	// Exécuter une requête SELECT pour récupérer tous les utilisateurs avec leurs informations
	rows, err := db.Query("SELECT username, created_at FROM users")
	if err != nil {
		log.Println("Erreur lors de la récupération des utilisateurs :", err)
		return err
	}
	defer rows.Close()

	type User struct {
		Username  string
		CreatedAt string // Modifier le type en string
	}

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.Username, &user.CreatedAt)
		if err != nil {
			log.Println("Erreur lors de la lecture des résultats de la requête :", err)
			return err
		}
		users = append(users, user)
	}

	// Créer une liste HTML des utilisateurs avec leurs informations supplémentaires
	userListHTML := "<h1>Liste des utilisateurs</h1><ul>"
	for _, user := range users {
		userListHTML += "<li>Utilisateur : " + user.Username + " | Date de création : " + user.CreatedAt + "</li>"
	}
	userListHTML += "</ul>"

	// Ajouter un lien vers la page d'accueil
	userListHTML += "<a href='/welcome'>Retour à la page </a>"

	return c.HTML(http.StatusOK, userListHTML)
}

// Gestionnaire pour la page d'inscription
func registerHandler(c echo.Context) error {
	return c.File("registerr.html")
}

// Traitement du formulaire d'inscription
func registerPostHandler(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	confirm_password := c.FormValue("confirm_password") // Récupérer le champ de confirmation du mot de passe
	// Vérifier si l'utilisateur existe déjà
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		log.Println("Erreur lors de la vérification de l'existence de l'utilisateur :", err)
		return err
	}
	if count > 0 {
		return c.File("userExisting.html")
	}

	// Vérifier si les mots de passe correspondent
	if password != confirm_password {
		return c.File("wrongMDP.html")
	}
	// Définir le rôle de l'utilisateur comme "utilisateur"
	role := "utilisateur"

	// Si l'utilisateur n'existe pas, insérer l'utilisateur dans la base de données
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("Erreur lors du hachage du mot de passe :", err)
		return err
	}

	insertQuery := "INSERT INTO users (username, password, role) VALUES (?, ?, ?)"
	_, err = db.Exec(insertQuery, username, hashedPassword, role)
	if err != nil {
		log.Println("Erreur lors de l'insertion dans la base de données :", err)
		return err
	}

	fmt.Printf("Utilisateur enregistré : %s\n", username)

	return c.File("successCreateUser.html")

}

// Page pour afficher le formulaire de suppression
func deleteFormHandler(c echo.Context) error {
	// HTML avec le formulaire de suppression et un bouton pour rediriger vers la page principale
	htmlContent := `
        <h1>Supprimer un utilisateur</h1>
        <form action='/delete' method='post'>
            <input type='text' name='username' placeholder='Username' required />
            <input type='password' name='password' placeholder='Password' required />
            <button type='submit'>Supprimer l'utilisateur</button>
        </form>
        <a href="/welcome">Retour à la page </a> <!-- Ajouter un lien vers la page principale -->
    `

	// Ajouter le code HTML pour la boîte de dialogue modale
	htmlContent += `
    <div id="myModal" class="modal">
      <div class="modal-content">
        <p>Êtes-vous sûr de vouloir supprimer votre compte ?</p>
        <button id="confirmDelete">Oui</button>
        <button id="cancelDelete">Non</button>
      </div>
    </div>
    `

	// Ajouter du JavaScript pour gérer l'affichage de la boîte de dialogue modale
	htmlContent += `
    <script>
    // Récupérer la boîte de dialogue modale
    var modal = document.getElementById("myModal");

    // Récupérer le bouton de confirmation et le bouton d'annulation
    var confirmBtn = document.getElementById("confirmDelete");
    var cancelBtn = document.getElementById("cancelDelete");

    // Ouvrir la boîte de dialogue modale lorsque l'utilisateur clique sur le bouton "Supprimer mon compte"
    document.getElementById("deleteAccountBtn").onclick = function() {
      modal.style.display = "block";
    }

    // Fermer la boîte de dialogue modale lorsque l'utilisateur clique sur le bouton d'annulation
    cancelBtn.onclick = function() {
      modal.style.display = "none";
    }

    // Rediriger vers la page de confirmation lorsque l'utilisateur clique sur le bouton de confirmation
    confirmBtn.onclick = function() {
      // Effectuer une requête AJAX pour supprimer le compte de l'utilisateur
      // Rediriger vers la page de confirmation
      window.location.href = "/goodbye";
    }
    </script>
    `

	// Vous pouvez ajouter d'autres éléments HTML ou fonctionnalités ici si nécessaire

	return c.HTML(http.StatusOK, htmlContent)
}

// Fonction pour supprimer un utilisateur
func deleteHandler(c echo.Context) error {
	// Récupérer le nom d'utilisateur et le mot de passe à partir du formulaire
	username := c.FormValue("username")
	password := c.FormValue("password")

	// Vérifier si l'utilisateur existe
	var storedPassword string
	err := db.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&storedPassword)
	if err != nil {
		// L'utilisateur n'existe pas dans la base de données
		log.Println("Erreur lors de la vérification de l'existence de l'utilisateur :", err)
		return c.HTML(http.StatusBadRequest, "<h1>Supprimer un utilisateur</h1><p>L'utilisateur n'existe pas dans la base de données.</p><a href='/delete'>Réessayer</a>")
	}

	// Vérifier si le mot de passe est correct en comparant avec le mot de passe haché stocké dans la base de données
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
	if err != nil {
		// Le mot de passe est incorrect
		log.Println("Mot de passe incorrect :", err)
		return c.HTML(http.StatusUnauthorized, "<h1>Supprimer un utilisateur</h1><p>Mot de passe incorrect.</p><a href='/delete'>Réessayer</a>")
	}

	// Le nom d'utilisateur et le mot de passe sont corrects, supprimer l'utilisateur de la base de données
	deleteQuery := "DELETE FROM users WHERE username = ?"
	_, err = db.Exec(deleteQuery, username)
	if err != nil {
		log.Println("Erreur lors de la suppression de l'utilisateur :", err)
		return err
	}

	fmt.Printf("Utilisateur supprimé : %s\n", username)

	return c.Redirect(http.StatusSeeOther, "/users") // Redirige vers la page des utilisateurs
}

// Page de déconnexion
func logoutHandler(c echo.Context) error {
	// Supprimer toutes les informations de session
	sess, _ := session.Get("session", c)
	sess.Options = &sessions.Options{MaxAge: -1} // Définir l'âge maximum de la session à -1 pour la supprimer
	sess.Save(c.Request(), c.Response())

	// Rediriger vers la page principale
	return c.Redirect(http.StatusSeeOther, "/")
}

// Gestionnaire de route pour visualiser le contenu du fichier
func viewFileHandler(c echo.Context) error {
	fileName := c.Param("fileName")

	// Décodage du nom de fichier
	decodedFileName, err := url.PathUnescape(fileName)
	if err != nil {
		return err
	}

	// Récupérer le chemin d'accès complet du fichier
	filePath := "uploads/" + decodedFileName

	// Lire le contenu du fichier
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Println("Erreur lors de la lecture du fichier :", err)
		return err
	}

	// Déterminer le type de contenu en fonction de l'extension du fichier
	contentType := http.DetectContentType(fileContent)

	// Retourner le contenu en fonction du type de fichier
	switch contentType {
	case "application/pdf":
		// Afficher le contenu PDF sur une nouvelle page
		return c.Blob(http.StatusOK, contentType, fileContent)
	case "image/png", "image/jpeg":
		// Afficher l'image sur une nouvelle page
		return c.Blob(http.StatusOK, contentType, fileContent)
	case "text/plain":
		// Afficher le contenu du fichier texte sur une nouvelle page
		return c.HTMLBlob(http.StatusOK, fileContent)
	default:
		// Gérer les autres types de fichiers (facultatif)
		return c.String(http.StatusOK, "Contenu du fichier non pris en charge")
	}
}

// Fonction pour supprimer un fichier de la base de données et du système de fichiers
func deleteFileHandler(c echo.Context) error {
	fileName := c.Param("fileName")

	// Supprimer le fichier de la base de données
	_, err := db.Exec("DELETE FROM files WHERE filename = ?", fileName)
	if err != nil {
		log.Println("Erreur lors de la suppression du fichier de la base de données :", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Erreur lors de la suppression du fichier de la base de données"})
	}

	// Supprimer le fichier du système de fichiers
	err = os.Remove("uploads/" + fileName)
	if err != nil {
		log.Println("Erreur lors de la suppression du fichier du système de fichiers :", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Erreur lors de la suppression du fichier du système de fichiers"})
	}

	// Répondre avec un code de succès
	return c.JSON(http.StatusOK, map[string]string{"message": "Fichier supprimé avec succès"})
}

func deleteAccountHandler(c echo.Context) error {
	// Récupérer l'ID de l'utilisateur à partir de la session
	userID, err := getUserIDFromSession(c)
	if err != nil {
		// Gérer l'erreur
		return err
	}

	// Supprimer le compte de l'utilisateur de la base de données
	_, err = db.Exec("DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		// Gérer l'erreur
		return err
	}

	// Supprimer toutes les autres informations de session associées à l'utilisateur
	sess, err := session.Get("session", c)
	if err != nil {
		// Gérer l'erreur
		return err
	}
	for key := range sess.Values {
		delete(sess.Values, key)
	}
	sess.Save(c.Request(), c.Response())

	// Rediriger l'utilisateur vers une page de confirmation ou une autre page appropriée
	return c.Redirect(http.StatusSeeOther, "/goodbye")
}
