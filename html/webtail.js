/*
  WebTail project js file
  https://github.com/LeKovr/webtail
*/

var WebTail = {
    uri: location.host + location.pathname + 'tail',
    file: '',         // log file name in tail mode
    every: 10,        // scroll after 0.01 sec
    wait: false,      // scroll activated
    scrolled: false,  // scroll allowed
    focused: false,   // window is not focused
    unread: 0,        // new rows when not focused
    title: '',        // page title
    timer: null,      // keepalive timer
    timeout: 5000     // ping & reconnect timeout

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
function showFiles(file) {
  WebTail.logs = file;
  var row = $('table.table tbody tr:last');
  var splitter = /^(.+)\/([^/]+)$/;
  var prevDir = '';
    var f = file;

    var p = row.clone();
    var path = '&nbsp;';
    var a = splitter.exec(f.name); // split file dir and name
    if (a == undefined) {
      name = f.name;
    } else {
      name = a[2];
      if (prevDir != a[1]) {
        path = prevDir = a[1];
      }
    }
    var item = $('*[data-file="'+file.name+'"]');

    if (item.length == 0) {
      row.before(p);
    } else {
      item.replaceWith(p);
//      $('#'+id).parent().replaceWith(p);
    }
    p.attr("data-file",file.name);
    p.find('[rel="path"]')[0].innerHTML = path;
    p.find('[rel="size"]')[0].innerHTML = sizeFormatted(f.size);
    p.find('[rel="mtime"]')[0].innerHTML = dateFormatted(f.mtime);
    if (f.size > 0) p.find('[rel="link"]').attr("href", '#' + f.name);
    p.find('[rel="link"]').text(name);
    p.removeClass('hide');
   
}

function titleReset() {
  if (WebTail.file != '') {
    document.title = WebTail.file + WebTail.title + ' - WebTail';
  } else {
    document.title = 'Log Index' + WebTail.title + ' - WebTail';
  }
}

function titleUnread(s) {
    if (s > 999) s = '***';
    document.title = '(' + s + ') ' + WebTail.file + WebTail.title + ' - WebTail';
}

// Start tail
function tail(file) {
    WebTail.file = file;
    titleReset();
    $('#tail-top').find('[rel="title"]')[0].innerHTML = file;
    var m = JSON.stringify({type: 'attach', channel: file})
    console.debug("send: "+m);
    WebTail.ws.send(m);
}
// WebTail.ws.send(JSON.stringify({type: 'stats'})

// Show files or tail page, reload on browser's back button
function showPage() {
  WebTail.unread = 0;
  WebTail.focused = true;
  if (location.hash == "") {
    $('table.table tbody').find("tr:not(:last)").remove();
    if (WebTail.file != '') {
      var m = JSON.stringify({type: 'detach', channel: WebTail.file});
      console.debug("send: "+m);
      WebTail.ws.send(m);
    }
    WebTail.file = '';
    titleReset();
    $('#tail-top').find('[rel="title"]')[0].innerHTML = '';
    $('#src').addClass('hide');
    $('#index').removeClass('hide');
    $('table.table thead tr:first').removeClass('hide'); // show header
    var m = JSON.stringify({type: 'attach'})
    console.debug("send: "+m);
    WebTail.ws.send(m);
  } else {
    $('#tail-data').text('');
    if (!$('#index').hasClass('hide')) {
      $('#index').addClass('hide');
      var m = JSON.stringify({type: 'detach'});
      console.debug("send: "+m);
      WebTail.ws.send(m);
  }
  $('#src').removeClass('hide');

    tail(location.hash.replace(/^#/,""));
  }
}

// Setup websocket
function connect() {
  try {

    var host = 'ws';
    if (window.location.protocol == 'https:') host = 'wss';
    host = host + '://' + WebTail.uri;
    WebTail.ws = new WebSocket(host);

    WebTail.ws.onopen = function() {
      showPage();
    }

    WebTail.ws.onclose = function(event) {
      if (event.wasClean) {
        console.debug('Connection closed clean');
      } else {
        console.debug('Connection aborted');
      }
      console.debug('Code: ' + event.code + ' reason: ' + event.reason);
      $("#log").text('Connection closed');
      if (WebTail.timer) {
        clearTimeout(WebTail.timer);
      }
      WebTail.timer = setTimeout(connect, WebTail.timeout);
    }

    WebTail.ws.onerror = function(e) {
      console.error(e.name + ': ' + e.message);
      $('#log').text(e.name);
    }

    WebTail.ws.onmessage = function(e) {
      console.debug("got "+e.data);
      var lines = e.data.split('\n');
      lines.forEach( function(l) {
        var m = JSON.parse(l, JSON.dateParser);
        $("#log").text('');

        if (m.type == 'index') {
          showFiles(m.data);
        } else if (m.type == 'attach') {
          // tail attached
        } else if (m.type == 'log') {
          var $area = $('#tail-data');
          $area.append(document.createTextNode((m.data != undefined)?m.data:''));
          $area.append("<br />");
          if (!WebTail.focused) {
            titleUnread(++WebTail.unread);
          }
          if (!WebTail.wait) {
            setTimeout(updateScroll,WebTail.every);
            WebTail.wait = true;
          }
        } else if (m.type == 'error') {
          console.warn("server error: %o", m);
        } else {
          console.warn("unknown response: %o", m);
        }
      });
    };

  } catch(e){
    console.log(e);
  }
}

// scroll window to bottom
function updateScroll(){
  if(!WebTail.scrolled){
    window.scrollTo(0,document.body.scrollHeight);
  }
  WebTail.wait = false;
}

$(function() {
  // Open websocket on start
  connect();
  WebTail.title = ' - ' + window.location.hostname;
  titleReset();

  $('#flag').click(function() {
    window.scrollTo(0,document.body.scrollHeight);
  });
});

$(window).scroll(function() {
  if ($(window).scrollTop() + $(window).height() == $(document).height()) {
     WebTail.scrolled = false; // user scrolled to bottom
     $('#flag').prop("disabled", true);
  } else if (!WebTail.wait) {
     WebTail.scrolled = true; // user scrolls up - switch autoscroll off
    $('#flag').prop("disabled", false);
  }
});

// Any click change hash => show page
window.onhashchange = function() {
  showPage();
}

// Set flag & clear title
window.onfocus = function() {
  WebTail.focused = true;
  titleReset();
};

// Reset flag
window.onblur = function() {
    WebTail.focused = false;
    WebTail.unread = 0;
};

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
