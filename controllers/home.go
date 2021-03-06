package controllers

import (
	"net/http"

	"github.com/nicomo/abacaxi/models"
	"github.com/nicomo/abacaxi/session"
	"github.com/nicomo/abacaxi/views"
)

// HomeHandler manages http requests on the home page
func HomeHandler(w http.ResponseWriter, r *http.Request) {

	// our messages (errors, confirmation, etc) to the user & the template will be store in this map
	d := make(map[string]interface{})

	// Get session
	sess := session.Instance(r)
	if sess.Values["id"] != nil {
		d["IsLoggedIn"] = true
	}

	// get flash messages if any
	if flashes := sess.Flashes(); len(flashes) > 0 {
		d["Flashes"] = flashes
	}
	sess.Save(r, w)

	// various stats about the data in the DB
	recordsCount := models.RecordsCount()
	d["recordsCount"] = recordsCount

	unimarcCount := models.RecordsCountUnimarc()
	d["unimarcCount"] = unimarcCount

	TSListing, _ := models.GetTargetServicesListing()
	d["TSListing"] = TSListing
	d["TSCount"] = len(TSListing)

	views.RenderTmpl(w, "home", d)
}
