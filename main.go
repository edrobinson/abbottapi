/*
    This go implements an interface to the Abbott Freestyle Lite
    glucose meter to capture it's stored readings to a file
    then generate a pdf of those readings
*/    

    package main
    
    import (
        "bytes"
        "bufio"
        "log"
        "net/http"
        "fmt"
        "io/ioutil"
        "github.com/jung-kurt/gofpdf"
        "os"
        "strings"
        "go.bug.st/serial" 
        "github.com/bmizerany/pat"
        "html/template"
        "time"
        
    )
 
    //Smbg is the structure passed to the 
    //PDF generator
    type smbg struct{
            smbgDate    string
            smbgTime    string
            smbgValue   string
    }

    //Instance the pdf generator
    var pdf = gofpdf.New("P", "in", "letter", "")
    
    //Vars for the date qualification functions
    var startdate time.Time //Earlier reference time
    var enddate   time.Time //Later reference time
    
    var datesentered bool           //True if either or both dates are submitted
    var startentered bool           //True if start date entered
    var endentered   bool           //True if end date entered
    var bothentered  bool           //True if both dates entered
    var nodates      bool           //True if no dates entered
    
    const shortDate = "2006-01-02" //Reference format
    
   
//Error checking centralied
func check(e error , msg string) {
    if e != nil {
        log.Fatal(msg,e)
    }
}    
//Main sets up the router and starts a listener
func main() {
    mux := pat.New()                          //Instance the pat router

    mux.Get("/", http.HandlerFunc(home))      //Render the home screen to gather user requests
    mux.Post("/", http.HandlerFunc(send))     //Process the submitted form values

    log.Println("Listening...")
    err := http.ListenAndServe(":3000", mux)  //Start a server instance and Listen on port 3000
    check(err,"Error on server start")        //Oops...
}

//Home renders the user input screen with form
func home(w http.ResponseWriter, r *http.Request) {
    render(w, "templates/abbottmain.html", nil)
}

//Send process the browser/user request 
func send(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()   //Get the form dates

    //1. Download the meter test results
    if !ReadAbbottResults(){
        check(nil,"Failed to read the meter data")
        os.Exit(2)
    }

    //2. Create and store a pdf of the test resulte
    createPDF("datafiles/abbottdata.txt", r.Form.Get("startdate"), r.Form.Get("enddate"))

    //Display the pdf in the browser
    showPDF(w, r, "abbott.pdf")

}

//ReformatDate formats the Abbott date from MMM  DD YYYY to YYYY-MM-DD
func reformatDate(s string) string{
    //Month name abbrevs
    var mos = []string{"000","Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"}
    
    //Month from the input date
    var month string = s[0:3]
    var imonth int
    for i, item := range mos {
        if item == month {
            imonth = i           
            break
        }
    }

    //Get the month as string
    month = fmt.Sprintf("%d",imonth)

    //Add leading zero if needed
    if len(month) < 2{
            month = "0" + month
    }
    var day string = s[4:7]
    var year string = s[7:]
    var newdate string = year + "-" + month + "-" + day   //Format: 2021-05-10
    newdate = strings.ReplaceAll(newdate," ", "")         //Remove all spaces
   //log.Println(newdate)
    return newdate 
}

/*
    Using the gofpdf package, create a pdf file from the 
    users measurments data
    The filename param is the file that contains the downloaded json.
    The pdf ge. object is instanced up top for global access
*/
//CreatePDF prepares the data and generates the PDF
func createPDF(filename, stdate, edate string){
    var smbgs []smbg        //Slice of smbg structures
    var psmbg smbg          //An smbg struct object
    f, err := os.Open("datafiles/abbottdata.txt")
        check(err, "Error loading result file")
    defer f.Close()

    //Initialize the date qualifier
 	_ = SetupDates(stdate, edate)
    
    scanner := bufio.NewScanner(f)
    
    //Sample result string
    //         1         2         3
    //123456789012345678901234567890  
    //178  May  03 2021 08:35 00 0x00
    
    //Convert the meter readings into
    //our smbg structure to feed the pdf generator
    for scanner.Scan() {
        //Get the next result and ckech for eof
        var s string = scanner.Text()
        if strings.Index(s, "END") > 0 {break}  //?last record then done
        
        //Check for a valid result string
        if len(s) < 25 {continue}               //? Not a test record then skip
       
        //Extract the date from the string and reformat it to YYYY-MM-DD
        var goodDate string = reformatDate(s[5:18])
        //Date in the user's specs?
        if !QualifyDate(goodDate) {continue} //out of user range?

        
        //Fill out the smbg structure
        psmbg.smbgDate  = goodDate
        psmbg.smbgTime  = s[18:24]                  //Time as HH:MM
        psmbg.smbgValue = strings.TrimSpace(s[0:5]) //Value as nnnnn

        //Append it to the smbg slice
        smbgs = append(smbgs,psmbg)
    }

    /*
        Now we are ready to produce the PDF.
        Initially I am creating a pretty basic PDF
        with no fancy page headings, etc.
        Stay tuned...
    */

    //SetHeaderFunc is the fpdf page header function
	pdf.SetHeaderFunc(func() {
		pdf.SetY(.2)
		pdf.SetFont("Arial", "B", 15)
		//pdf.Cell(2.2, 0, "")
		pdf.CellFormat(0, .4, "Glucose Values", "", 0, "C", false, 0, "")
        pdf.Ln(.2)
        dt := time.Now()
        dts := dt.Format("01-02-2006 15:04")
        pdf.SetFont("Arial", "", 12)        
        pdf.CellFormat(0, .4, dts, "", 0, "C", false, 0, "")
		pdf.Ln(.5)
        //Add the column headers
        lineOut("Date","Time","Glucose mg/dl")
        
	})
	
    //SetFooterFunc the fpdf page footer function
    pdf.SetFooterFunc(func() {
        pdf.SetY(-.5)
		pdf.SetFont("Arial", "I", 8)
		pdf.CellFormat(0, .4, fmt.Sprintf("Page %d /{nb}", pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
   
    pdf.AliasNbPages("")
    pdf.AddPage()                               //Put on the first page    
    pdf.SetFont("Arial", "", 12)                //Set the document font
    
    
    //Add all of the measurements
    for i := range smbgs{
        lineOut(smbgs[i].smbgDate, smbgs[i].smbgTime, smbgs[i].smbgValue)
    }
    
    //Store the pdf file and cleanup
    pdf.OutputFileAndClose("abbott.pdf")
}

//LineOut - Output a result line of cells to the pdf
func lineOut(s1, s2, s3 string){
        pdf.Cell(1.35,0,"")    //1" indent
        cellOut(s1)
        cellOut(s2)
        cellOut(s3)
        pdf.Ln(0.3)
}        

//CellOut - Standardize the cell format
func cellOut(s string){
    pdf.CellFormat(1.7,0.3,s,"1",0,"C",false,0,"")
}

//ReadAbbottResults implements the Abbott Freestyle Lite
//data download interface. It reads the test result
//records from the meter via a usb serial port
//and writes them to a text file for later use
//by PDF creation function
func ReadAbbottResults() bool{
    
    //Open a file to store the meter data
    
    f, err := os.Create("datafiles/abbottdata.txt")
    defer f.Close()
    
    
    
	//Set the port mode vars
	mode := &serial.Mode{
		BaudRate: 19200,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}
    //Open COM5 using the mode
	port, err := serial.Open("COM5", mode)
	if err != nil {
		return false
	}

	//Write the glucose meter trigger string "mem"
    //to start the output of it's stored results
	n,err := port.Write([]byte("mem"))
    
	if err != nil {
		return false
	}
	fmt.Printf("Sent %v bytes\n", n)

	// Read and store the response lines
    // until the line containing "END" is received
    //At that point return
    //Goal is to write them to a file
	buff := make([]byte, 100)
	for {
		// Reads up to 100 bytes
		n, err := port.Read(buff)
		if err != nil {
			return false
		}
		if n == 0 {
			fmt.Println("\nEOF")
			return true
		}
        
        //Get the buffer to a string
        var s = fmt.Sprintf("%s",buff[:n])
       
       //Write it to our data file
        f.WriteString(s)
       
       //Check for the data end flag
        if strings.Contains(s, "END"){return true}
	}
}
//ShowPDF - Render the completion page
func showPDF(w http.ResponseWriter, r *http.Request, filename string){
        streamPDFbytes, err := ioutil.ReadFile( filename )
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
        b := bytes.NewBuffer(streamPDFbytes)
        w.Header().Set("Content-type", "application/pdf")
        if _, err := b.WriteTo(w); err != nil {
            fmt.Fprintf(w, "%s", err)
        }
}


//Render - Load and Render the HTML to the browser
//Called by the pat events
func render(w http.ResponseWriter, filename string, data interface{}) {
    //Load and parse the html file
    tmpl, err := template.ParseFiles(filename)
    if err != nil {
    log.Println(err)
    http.Error(w, "Sorry, something went wrong", http.StatusInternalServerError)
    }

    //Output the html to the browser
    if err := tmpl.Execute(w, data); err != nil {
    
    log.Println(err)
    http.Error(w, "Sorry, something went wrong", http.StatusInternalServerError)
    }
} 

//----------------------------- Date qualification functions ------------------------------

//ShortDateFromString parse short date from string as YYYY-MM-DD
func ShortDateFromString(ds string) (time.Time, error) {
    t, err := time.Parse(shortDate, ds)
    if err != nil {
        return t, err
    }
    return t, nil
}

//SetupDates initializes the date qualification mechannism - takes both user date values,
//stores them and detrmines if either of them was not blank  
//Returns false if both dates = "" 
//If false is returned then no need to make further calls 
func SetupDates(startd, endd string) bool{
    //Either starting dates sent?
    if startd == "" && endd == ""{
        datesentered = false
        nodates = true
        return false
    }else{
        datesentered = true
        nodates = false
    }
        
    //If a start date string was sent, parse it 
    //and note its presence
    startentered = false
    if startd != ""{
        startdate,_ = ShortDateFromString(startd)
        startentered = true
        
    }
    
    //Same for the end date
    endentered = false
    if endd != ""{
        enddate,_ = ShortDateFromString(endd)
        endentered = true
    }

    bothentered = false
    if startentered && endentered {bothentered = true}
    return true
}

//QualifyDate - See if the passed date qualifies
//Checks for both dates, start only and end only
func QualifyDate(dt string)bool{
    if !datesentered {return true} //Shouldn't happen but...
    
    d, _ := ShortDateFromString(dt)
    
    //1. both dates entered. See if this date fits the range
    if bothentered{
        if d.After(startdate) &&  d.Before(enddate) {
            return true
        }else{
            return false
        }
    }
    
    //2. Only the start date sent
    if startentered{
        if d.Before(startdate) {
            return true
        }else{
            return false
        }
    }
        
    //3. Only end date enered
    if endentered{   
        if d.After(enddate){
            return true
        }else{
            return false
        }
    }
    return false
}    
    