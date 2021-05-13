# abbottapi
 Abbott Freestyle Lite Interface
 
 This is a Go app that downloads blood glucose results from the
 Abbott Freestyle Lite Meter.
 
 The interface is very simple:
 
 1. The connection uses a USB serial port cable.
 2. The code sends a string - mem - to the meter to trigger the download.
 3. The meter sents each stored result as a CRLF delimited string.
 4. The end of the download is singaled by a record containing the string "END".
 5. The data records are written to a text file.

The code then uses the repository "github.com/jung-kurt/gofpdf" to make and
store a PDF of the results.

The code then displays the PDF in the browser.

This is a web app of sorts. It uses Go to start aa server on port 3000
and displays a web page when accessed. The page accepts optional start
and end dates to filter the results.

I use Win-10 but it should work in other environments...
