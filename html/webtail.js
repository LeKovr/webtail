/*
  WebTail project js file
  https://github.com/LeKovr/webtail
*/

var WebTail = {
    uri: location.host + '/tail',
    every: 500,
    timeInMs: Date.now()
    // ws
    // logs
};

// Format datetime
// code from http://stackoverflow.com/a/32062237
// with changed result formatting
function dateFormatted(date) {
  var month = date.getMonth() + 1;
  var day = date.getDate();
  var hour = date.getHours();
  var min = date.getMinutes();
  var sec = date.getSeconds();

  month = (month < 10 ? "0" : "") + month;
  day = (day < 10 ? "0" : "") + day;
  hour = (hour < 10 ? "0" : "") + hour;
  min = (min < 10 ? "0" : "") + min;
  sec = (sec < 10 ? "0" : "") + sec;

  var str = day + "." + month + "." + date.getFullYear() + " " +  hour + ":" + min + ":" + sec;
  return str;
}

// Format file size
// code from http://stackoverflow.com/a/20732091
// with fix for zero size
function sizeFormatted(size) {
  if (!size) return '0 B';
  var i = Math.floor( Math.log(size) / Math.log(1024) );
  return ( size / Math.pow(1024, i) ).toFixed(2) * 1 + ' ' + ['B', 'kB', 'MB', 'GB', 'TB'][i];
}

// Show table with logfiles list
function showFiles(files) {
  WebTail.logs = files;
  $('table.table thead tr:first').removeClass('hide'); // show header
  var row = $('table.table tbody tr:last');
  var splitter = /^(.+)\/([^/]+)$/;
  var prevDir = '';

  var sorted = Object.keys(files).sort().forEach( function(s) {
    var f = files[s];
    var p = row.clone();
    var path = '&nbsp;';
    row.before(p);
    var a = splitter.exec(s); // split file dir and name
    if (a == undefined) {
      name = s;
    } else {
      name = a[2];
      if (prevDir != a[1]) {
        path = prevDir = a[1];
      }
    }
    p.find('[rel="path"]')[0].innerHTML = path;
    p.find('[rel="size"]')[0].innerHTML = sizeFormatted(f.size);
    p.find('[rel="mtime"]')[0].innerHTML = dateFormatted(f.mtime);
    p.find('[rel="link"]').attr("href", '#' + s).text(name);
    p.removeClass('hide');
  } );
}

// Start tail
function tail(file) {
    $('#copied').text('');
    $('#src').find('[rel="title"]')[0].innerHTML = file;
    $('#src').clone().attr("id", "tail-data").appendTo('#copied').removeClass('hide');
    var m = JSON.stringify({channel: file})
    console.debug("send: "+m);
    WebTail.ws.send(m);
}

// Show files or tail page, reload on browser's back button
function showPage() {
    if (location.hash == "") {
      if (WebTail.logs != undefined) {
        location.reload(true); // TODO: reopen socket
      }
      var m = '{}';
      console.debug("send: "+m);
      WebTail.ws.send(m);
    } else {
      tail(location.hash.replace(/^#/,""));
    }
}

// Setup websocket
function connect() {
  try {
    var host = 'ws://' + WebTail.uri;
    WebTail.ws = new WebSocket(host);

    WebTail.ws.onopen = function() {
      if (WebTail.logs == undefined) {
        showPage();
      }
    }

    WebTail.ws.onclose = function() {
      $("#log").text('Connection closed');
    }

    WebTail.ws.onerror = function(e) {
      $('#log').text(e.name);
    }

    WebTail.ws.onmessage = function(e) {
      var m = JSON.parse(e.data, JSON.dateParser);
      console.debug("got "+e.data);
      $("#log").text('');

      if (m.channel == undefined) {
        showFiles(m.message);
      } else {
        var $area = $('#tail-data .data');
        $area.append(m.message + "\n");
        if (Date.now() - WebTail.timeInMs > WebTail.every) {
          $area.scrollTop($area[0].scrollHeight - $area.height());
          WebTail.timeInMs = Date.now();
        }
      }
    };

  } catch(e){
    console.log(e);
  }
}

// Open websocket on start
$(function() {
  connect();
});

// Any click change hash => show page
window.onhashchange = function() {
  showPage();
}

// JSON datetime parser
// code from https://weblog.west-wind.com/posts/2014/jan/06/javascript-json-date-parsing-and-real-dates
// with fix for integer seconds
if (window.JSON && !window.JSON.dateParser) {
  var reISO = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2}(\.\d*)?)(?:Z|(\+|-)([\d|:]*))?$/;
  var reMsAjax = /^\/Date\((d|-|.*)\)[\/|\\]$/;

  JSON.dateParser = function (key, value) {
    if (typeof value === 'string') {
      var a = reISO.exec(value);
      if (a)
        return new Date(value);
      a = reMsAjax.exec(value);
      if (a) {
        var b = a[1].split(/[-+,.]/);
        return new Date(b[0] ? +b[0] : 0 - +b[1]);
      }
    }
    return value;
  };
}
