window.beforeunload = function (e) {
  e.preventDefault();
  return (e.returnValue = "Are you sure you want to exit?");
};
let position = 0;
let chosenRune = "";
const pfObj = {
  0: "3*W",
  7: "3*W",
  14: "3*W",
  105: "3*W",
  112: "3'W",
  119: "3*W",
  210: "3'W",
  217: "3*W",
  224: "3'W",
  16: "2*W",
  28: "2*W",
  32: "2*W",
  42: "2*W",
  48: "2*W",
  56: "2*W",
  154: "2*W",
  160: "2*W",
  168: "2*W",
  176: "2*W",
  182: "2*W",
  192: "2*W",
  3: "2*B",
  196: "2*W",
  208: "2*W",
  3: "2*B",
  11: "2*B",
  36: "2*B",
  38: "2*B",
  45: "2*B",
  52: "2*B",
  59: "2*B",
  92: "2*B",
  96: "2*B",
  98: "2*B",
  102: "2*B",
  108: "2*B",
  116: "2*B",
  122: "2*B",
  126: "2*B",
  128: "2*B",
  132: "2*B",
  165: "2*B",
  172: "2*B",
  179: "2*B",
  186: "2*B",
  188: "2*B",
  213: "2*B",
  221: "2*B",
  20: "3*B",
  24: "3*B",
  76: "3*B",
  80: "3*B",
  84: "3*B",
  88: "3*B",
  136: "3*B",
  140: "3*B",
  144: "3*B",
  148: "3*B",
  200: "3*B",
  20: "3*B",
  204: "3*B",
};
const letterPoints = {
  Q: 10,
  Y: 10,
  Ö: 8,
  X: 8,
  Ä: 6,
  J: 6,
  Ü: 6,
  V: 6,
  P: 4,
  C: 4,
  F: 4,
  K: 4,
  B: 3,
  M: 3,
  W: 3,
  Z: 3,
  H: 2,
  G: 2,
  L: 2,
  O: 2,
  N: 1,
  E: 1,
  S: 1,
  I: 1,
  R: 1,
  T: 1,
  U: 1,
  A: 1,
  D: 1,
  "*": 0,
};
const validInputs = [
  "A",
  "B",
  "C",
  "D",
  "E",
  "F",
  "G",
  "H",
  "I",
  "J",
  "K",
  "L",
  "M",
  "N",
  "O",
  "P",
  "Q",
  "R",
  "S",
  "T",
  "U",
  "V",
  "W",
  "X",
  "Y",
  "Z",
  "Ä",
  "Ö",
  "Ü",
];
const playfield = document.getElementById("pfield");
const menu = document.getElementById("menu");
const msgs = document.getElementById("msgs");
const iAm = document.getElementById("player").value;
let currentPlayer = false;
let currentLet = false;
let currentParentLet = false;
let msgCount = 0;
let myMoves = [];
let isRunning = false;
const ar = [];
let x = 0;
//let markedEl = false;
for (let i = 0; i < 15; i++) {
  let innerStr = '<div class="week">';
  for (let k = 0; k < 15; k++) {
    innerStr +=
      '<div class="item' +
      (pfObj["" + x] ? " bg" : "") +
      '" data-id="' +
      x +
      '">' +
      (pfObj["" + x] || "") +
      "</div>";
    x++;
  }
  innerStr += "</div>";
  ar.push(innerStr);
}
playfield.innerHTML = ar.join("");

menu.innerHTML +=
  '<div class="day bgb">&copysr;1&check;</div><div class="day bgb">&copysr;2&check;</div>' +
  '<div class="day bgb">&copysr;3&check;</div><div data-do="swap" class="day bgbb">&iquest;</div>' +
  '<div class="day letter"></div><div class="day letter">' +
  '</div><div class="day letter"></div><div class="day letter"></div><div class="day letter">' +
  '</div><div class="day letter"></div><div class="day letter"></div>' +
  '<div id="currentlet" class="day bgb"></div><div class="day bgbb" title="Rückgängig" data-do="undo">&#10008;</div>' +
  '<div class="day bgbb" title="Fertig gelegt" data-do="play">&#10004;</div><div title="Start Game" id="sgame" class="day bgbb">&blacktriangleright;</div>';

currentLet = document.getElementById("currentlet");
let stream = new EventSource("/sse");
stream.addEventListener("message", function (e) {
  try {
    const p = JSON.parse(e.data);
    console.log(p);
    if(p.type && p.type === "chat") {
      printMsg(p.from + ": " + sanitizeHTML(p.txt));
    } else {
      console.log(e.data);
      printMsg(e.data);
    }
  } catch (err) {
    console.log(err);
  }
  // curp points letters
});
stream.addEventListener("stat", function (e) {
  try {
    const s = JSON.parse(e.data);
    currentPlayer = s.curp;
    console.log(typeof s.started);
    if (s.started) {
      if (iAm == currentPlayer) {
        document.getElementById("ami").style.display = "block";
      }
      s.fields?.forEach(
        (f) =>
          (playfield.querySelector(
            '[data-id="' + f.position + '"]'
          ).innerHTML += '<div class="played">' + f.rune + "</div>")
      );
    }
    const runes = s.letters.split("").sort();
    const els = menu.querySelectorAll("div.day.letter");
    let i = 0;
    runes.forEach(function (r) {
      if (runes[i] != "|") {
        els[i].innerHTML =
          '<div class="played" data-do="mark">' + runes[i] + "</div>";
      } else {
        els[i].innerHTML = "";
      }
      i++;
    });
  } catch (err) {
    console.log(err);
  }
});
stream.addEventListener("next", function (e) {
  try {
    let obj = JSON.parse(e.data);
    if (obj.hasOwnProperty("points") && obj.hasOwnProperty("words")) {
      printMsg(
        "Server: player" +
          obj.cp +
          (obj.words.length ? " hat die Worte " + obj.words + " gelegt." : "") +
          " Seine Punkte: " +
          obj.points
      );
    }
    currentPlayer = obj.player;
    if (iAm == currentPlayer) {
      newToast({ html: "Du bist an der Reihe!" });
      document.getElementById("ami").style.display = "block";
    } else {
      newToast({ html: obj.player + " ist an der Reihe." });
      document.getElementById("ami").style.display = "none";
    }
    if (obj.hasOwnProperty("fields")) {
      obj.fields.forEach(
        (f) =>
          (playfield.querySelector(
            '[data-id="' + f.position + '"]'
          ).innerHTML += '<div class="played">' + f.rune + "</div>")
      );
    }
  } catch (err) {
    console.log(err);
  }
});
stream.addEventListener("end", function (e) {
  isRunning = false;
  try {
    let obj = JSON.parse(e.data);
    if (obj.hasOwnProperty("points") && obj.hasOwnProperty("words")) {
      printMsg(
        "Server: player" +
          obj.cp +
          (obj.words.length ? " hat die Worte " + obj.words + " gelegt." : "") +
          " Seine Punkte: " +
          obj.points
      );
    }
    currentPlayer = obj.player;
    newToast({
      html: "Das Spiel ist beendet. Eine neue Runde kann von Player1 gestartet werden!",
    });
    document.getElementById("ami").style.display = "none";

    if (obj.hasOwnProperty("fields")) {
      obj.fields.forEach(
        (f) =>
          (playfield.querySelector(
            '[data-id="' + f.position + '"]'
          ).innerHTML += '<div class="played">' + f.rune + "</div>")
      );
    }
  } catch (err) {
    console.log(err);
  }
});
stream.addEventListener("start", function (e) {
  let s = "";
  try {
    s = JSON.parse(e.data);
  } catch (err) {
    console.log(err);
  }
  newToast({ html: "Das Spiel ist gestartet. " + s.p + " beginnt!" });
  currentPlayer = s.p;
  if (iAm == currentPlayer) {
    document.getElementById("ami").style.display = "block";
  } else {
    document.getElementById("ami").style.display = "none";
  }
  isRunning = true;
});
stream.addEventListener("runes", function (e) {
  try {
    const p = JSON.parse(e.data);
    const runes = p.runes.split("").sort();
    const els = menu.querySelectorAll("div.day.letter");
    for (let i = 0; i < 7; i++) {
      els[i].innerHTML =
        '<div class="played" data-do="mark">' + runes[i] + "</div>";
    }
  } catch (err) {
    console.log(err);
  }
});
stream.addEventListener("chat", function (e) {
  try {
    const p = JSON.parse(e.data);
    printMsg(p.from + ": " + sanitizeHTML(p.txt));
  } catch (err) {
    console.log(err);
  }
});

function sanitizeHTML(str) {
  return str.replace(/[^\w. ]/gi, function (c) {
    return "&#" + c.charCodeAt(0) + ";";
  });
}

window.addEventListener("load", function () {
  dragula({
    isContainer: function (el) {
      return el.classList.contains("letter") || el.classList.contains("item");
    },
    moves: function (el, container, handle, sibling) {
      return (
        el.classList.contains("played") &&
        container.classList.contains("letter")
      );
    },
    accepts: function (el, target, source, sibling) {
      //console.log(target);
      if (target.classList.contains("letter")) {
        return false;
      }
      let x = target.querySelectorAll("div.played").length;
      if (x > 1) {
        return false;
      }
      // now put dropped position and so on into myMoves...
      return true;
    },
    copy: false, // elements are moved by default, not copied
    copySortSource: false, // elements in copy-source containers can be reordered
    revertOnSpill: true, // spilling will put the element back where it was dragged from, if this is true
    removeOnSpill: false, // spilling will `.remove` the element, if this is true
    mirrorContainer: document.body, // set the element that gets mirror elements appended
    ignoreInputTextSelection: true, // allows users to select input text
    slideFactorX: 5, // allows users to select the amount of movement on the X axis before it is considered a drag instead of a click
    slideFactorY: 5,
  }).on("drop", function (el, target, source, sibling) {
    let position = parseInt(target.getAttribute("data-id"), 10);
    let cRune = el.innerText;
    let joker = false,
      input;
    // occupied already?
    if (el.innerText === "*") {
      joker = true;
      input = prompt(
        "Bitte den Buchstaben eingeben, den der Joker repräsentiert:"
      );
      input = input.toUpperCase();
      while (!validInputs.includes(input)) {
        input = prompt(
          "Bitte einen validen Buchstaben eingeben, den der Joker repräsentiert:"
        ).toUpperCase();
      }
      el.innerText = "*" + input.toUpperCase();
    }
    el.classList.add("current");
    currentParentLet = false;
    myMoves.push({
      position: position,
      rune: joker ? input : cRune,
      joker: joker,
    });
  });
});

if (iAm == "player1") {
  document.getElementById("sgame").setAttribute("data-do", "start-game");
}
document.getElementById("msg-btn").addEventListener("click", function (ev) {
  this.disabled = true;
  const msg = document.getElementById("txt").value;

  if (msg) {
    postApi("/chat/msg", JSON.parse(JSON.stringify({ msg: msg})))
      .then((resp) => {
        if (resp.error) {
          return newToast({
            html: resp.error,
            background: "red",
            color: "white",
          });
        }
      })
      .catch((err) => {
        return newToast({
          html: err.message || err,
          background: "red",
          color: "white",
        });
      })
      .finally(() => {
        this.disabled = false;
        document.getElementById("txt").value = "";
      })
  }
});
document.getElementById("savebtn").addEventListener("click", function (ev) {
  let inp = document.getElementById("wortinput").value;
  if (inp) {
    let war = inp.toLowerCase().split(",");
    postApi("/word/add", { words: war })
      .then((resp) => {
        if (resp.error) {
          return newToast({
            html: resp.error,
            background: "red",
            color: "white",
          });
        } else {
          return newToast({ html: "erfolgreich gespeichert!" });
        }
      })
      .catch((err) => {
        return newToast({
          html: err.message || err,
          background: "red",
          color: "white",
        });
      })
      .finally(() => {
        document.getElementById("wortinput").value = "";
      });
  }
});
document.getElementById("delbtn").addEventListener("click", function (ev) {
  let inp = document.getElementById("wortinput").value;
  if (inp) {
    let war = inp.toLowerCase().split(",");
    postApi("/word/remove", { words: war })
      .then((resp) => {
        if (resp.error) {
          return newToast({
            html: resp.error,
            background: "red",
            color: "white",
          });
        } else {
          return newToast({ html: "erfolgreich gelöscht!" });
        }
      })
      .catch((err) => {
        return newToast({
          html: err.message || err,
          background: "red",
          color: "white",
        });
      })
      .finally(() => {
        document.getElementById("wortinput").value = "";
      });
  }
});

function swapLetters() {
  if (isRunning !== true) {
    return newToast({
      html: "Das Spiel ist schon zuende!",
      background: "red",
      color: "white",
    });
  }
  if (iAm == currentPlayer) {
    if (myMoves.length) {
      return newToast({
        html: "Buchstaben sollten nur getauscht werden, wenn noch nichts gelegt wurde!",
        background: "red",
        color: "white",
      });
    }
    postApi("/letter/swap", {})
      .then((resp) => {
        if (resp.error) {
          return newToast({
            html: resp.error,
            background: "red",
            color: "white",
          });
        } else {
          let els = menu.querySelectorAll("div.day.letter");
          if (resp.letters) {
            let i = 0;
            resp.letters
              .split("")
              .sort()
              .forEach((it) => {
                els[i].innerHTML =
                  '<div class="played" data-do="mark">' + it + "</div>";
                i++;
              });
          }
        }
      })
      .catch((err) => {
        return newToast({
          html: err,
          background: "red",
          color: "white",
        });
      })
      .finally(() => {
        // enable buttons
      });
  }
}
function startIt() {
  if (iAm == "player1") {
    getApi("/game/start")
      .then((resp) => {
        if (resp.error) {
          return newToast({
            html: resp.error,
            background: "red",
            color: "white",
          });
        }
      })
      .catch((err) => {
        return newToast({
          html: err.message || err,
          background: "red",
          color: "white",
        });
      })
      .finally(() => {});
  }
}
function playIt() {
  if (isRunning !== true) {
    return newToast({
      html: "Das Spiel ist schon zuende!",
      background: "red",
      color: "white",
    });
  }
  const els = menu.querySelectorAll("div.day.letter");
  let toBePut;
  if (iAm === currentPlayer) {
    postApi("/game/play", myMoves)
      .then((resp) => {
        if (resp.msg && resp.msg === "ok wait for next player") {
          return;
        }
        if (resp.error) {
          toBePut = myMoves.map((it) => (it.joker ? "*" : it.rune));
          myMoves.forEach((it) => {
            let top = playfield.querySelector(
              '[data-id="' + it.position + '"]'
            );
            let el = top.querySelector("div.played");
            top.removeChild(el);
          });
          newToast({
            html: resp.error,
            background: "red",
            color: "white",
          });
        } else {
          toBePut = resp.letters.split("").sort();
          const sW = JSON.parse(resp.words);
          newToast({
            html: "Gut gemacht! Deine Worte:<br>" + sW.join("<br>"),
          });
          if (resp.points) {
            document.getElementById("points").innerText =
              resp.points + " Punkte";
          }
        }
        toBePut.forEach((it) => {
          for (let i = 0; i < 7; i++) {
            if (els[i].querySelector("div.played")) {
              continue;
            } else {
              els[i].innerHTML =
                '<div class="played" data-do="mark">' + it + "</div>";
              break;
            }
          }
        });
      })
      .catch((err) => {
        toBePut = myMoves.map((it) => (it.joker ? "*" : it.rune));
        myMoves.forEach((it) => {
          let top = playfield.querySelector('[data-id="' + it.position + '"]');
          let el = top.querySelector("div.played");
          top.removeChild(el);
        });
        toBePut.forEach((it) => {
          for (let i = 0; i < 7; i++) {
            if (els[i].querySelector("div.played")) {
              continue;
            } else {
              els[i].innerHTML =
                '<div class="played" data-do="mark">' + it + "</div>";
              break;
            }
          }
        });
        return newToast({
          html: err.message || err,
          background: "red",
          color: "white",
        });
      })
      .finally(() => {
        myMoves = [];
        [].forEach.call(
          playfield.querySelectorAll("div.current"),
          function (it) {
            it.className = "played";
          }
        );
      });
  } else {
    toBePut = myMoves.map((it) => (it.joker ? "*" : it.rune));
    myMoves.forEach((it) => {
      let top = playfield.querySelector('[data-id="' + it.position + '"]');
      let el = top.querySelector("div.played");
      top.removeChild(el);
    });
    newToast({
      html: "Hey, du bist gerade nicht an der Reihe...!",
      background: "red",
      color: "white",
    });
  }
}
menu.addEventListener("click", function (ev) {
  const toDo = ev.target.getAttribute("data-do");
  if (toDo) {
    switch (toDo) {
      case "swap":
        swapLetters();
        break;
      case "start-game":
        startIt();
        break;
      case "mark":
        chosenRune = ev.target.innerText;
        currentLet.innerText = chosenRune;
        currentParentLet = ev.target;
        break;
      case "play":
        playIt();
        break;
      case "undo":
        const x = myMoves.pop();
        if (!x) {
          return;
        }
        let top = playfield.querySelector('[data-id="' + x.position + '"]');
        let el = top.querySelector("div.played");
        top.removeChild(el);
        const els = menu.querySelectorAll("div.day.letter");
        for (let i = 0; i < 7; i++) {
          if (els[i].querySelector("div.played")) {
            continue;
          } else {
            els[i].innerHTML =
              '<div class="played" data-do="mark">' +
              (x.joker ? "*" : x.rune) +
              "</div>";
            break;
          }
        }
        break;
    }
  }
});

function getApi(url) {
  const requestOptions = {
    method: "GET",
  };
  return fetch(url, requestOptions).then(handleResponse);
}

function postApi(url, body) {
  const csrf = document.getElementById("csrf").value;
  const requestOptions = {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-CSRF-TOKEN": csrf },
    body: JSON.stringify(body),
  };
  return fetch(url, requestOptions).then(handleResponse);
}

function handleResponse(response) {
  return response.text().then((text) => {
    const data = text && JSON.parse(text);

    if (!response.ok) {
      const error = (data && data.error) || response.statusText;
      return Promise.reject(error);
    }
    return data;
  });
}

function printMsg(msg) {
  msgs.innerHTML = "<p>" + msg + "</p>" + msgs.innerHTML;
  msgCount++;
  while (msgCount > 100) {
    msgs.removeChild(msgs.lastChild);
    msgCount--;
  }
}

function newToast(opts = {}) {
  let options = {
    ...{
      margin: 15,
      duration: 3000,
      html: "Hello World",
      background: "green",
      color: "white",
    },
    ...opts,
  };
  const newMsg = document.createElement("div");
  newMsg.className = "os-toast";
  newMsg.innerHTML = options.html;
  newMsg.style.position = "fixed";
  newMsg.style.top = "100px";
  newMsg.style.right = "15px";
  newMsg.style.transform = "scale(1)";
  newMsg.style.opacity = 1;
  newMsg.style.backgroundColor = options.background;
  newMsg.style.color = options.color;
  document.body.insertBefore(newMsg, document.body.firstChild);

  setTimeout(() => {
    hide(newMsg);
  }, options.duration);

  let pushStack = options.margin;
  Array.from(document.querySelectorAll(".os-toast"))
    .filter((t) => t.parentElement === newMsg.parentElement)
    .forEach((t) => {
      t.style.top = pushStack + "px";
      pushStack += t.offsetHeight + options.margin;
    });
}

function hide(el) {
  el.style.opacity = 0;
  const tEnd = () => {
    el.parentElement.removeChild(el);
    el.removeEventListener("transitionend", tEnd);
  };
  el.addEventListener("transitionend", tEnd);
}
