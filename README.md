# wordgame
A wordgame with a golang backend including german words checker

## Word Database Used
[germandict](https://sourceforge.net/projects/germandict/)

I created a NodeJS script like so:
var iconv = require('iconv-lite');
var fs = require('fs');
fs.createReadStream('german.dic')
    .pipe(iconv.decodeStream('latin1'))
    .pipe(iconv.encodeStream('utf8'))
    .pipe(fs.createWriteStream('german.txt'));

to convert into utf8. Importing into sqlite was then done by parsing each line from Golang and converting everything into lower case to circumvent search issues with German Umlaute in sqlite.

## How to use this repo
Important - on first run provide your own passwords for users, there is nothing they can right now do except for playing, but in the future there maybe an admin area doing security related things. 

Start it from the command line by cd-ing into root folder and typing go run ./cmd or cd into cmd folder and issue go build -o ../app to produce binary that can be started from the correct folder to be able to parse the templates.

Player1 is the one who can start a game for the moment. 

Every player can help customize the database of words and help make it your version of the game, on the left of the screen after logging in there is a textbox and two buttons - to save or remove words from the database.

Above those players find a chat function and a box with messages from the server / chat.

## To dos
Remove special characters reliably from chat messages to make them valid JSON in any case
Multiple rooms/games in parallel
...
