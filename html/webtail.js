/*
  WebTail project js file
  https://github.com/LeKovr/webtail
*/

var WebTail = {
    uri: location.host + location.pathname + 'tail',
    file: '', // log file name in tail mode
    every: 10, // scroll after 0.01 sec
    wait: false, // scroll activated
    scrolled: false, // scroll allowed
    focused: false, // window is not focused
    unread: 0, // new rows when not focused
    title: '', // page title
    timer: null, // keepalive timer
    timeout: 5000, // ping & reconnect timeout
    attached: null // attached channel
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

    var str = day + "." + month + "." + date.getFullYear() + " " + hour + ":" + min + ":" + sec;
    return str;
}

// Format file size
// code from http://stackoverflow.com/a/20732091
// with fix for zero size
function sizeFormatted(size) {
    if (!size) return '0 B';
    var i = Math.floor(Math.log(size) / Math.log(1024));
    return (size / Math.pow(1024, i)).toFixed(2) * 1 + ' ' + ['B', 'kB', 'MB', 'GB', 'TB'][i];
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
    if (a == undefined) { // '===' does not works
        p.find('[rel="link"]').text(f.name);
    } else {
        p.find('[rel="link"]').text(a[2]);
        if (prevDir !== a[1]) {
            path = prevDir = a[1];
        }
    }
    var item = $('*[data-file="' + file.name + '"]');

    if (item.length === 0 && f.deleted) {
        return;
    } else if (f.deleted) {
        item.remove()
    } else if (item.length === 0) {
        row.before(p);
    } else {
        item.replaceWith(p);
        //      $('#'+id).parent().replaceWith(p);
    }
    p.attr("data-file", file.name);
    p.find('[rel="path"]')[0].innerHTML = path;
    p.find('[rel="size"]')[0].innerHTML = sizeFormatted(f.size);
    p.find('[rel="mtime"]')[0].innerHTML = dateFormatted(f.mtime);
    if (f.size > 0) p.find('[rel="link"]').attr("href", '#' + f.name);
    p.removeClass('hide');
}

function titleReset() {
    if (WebTail.file !== '') {
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
    var m = JSON.stringify({ type: 'attach', channel: file })
    window.console.debug("send: " + m);
    WebTail.ws.send(m);
}

// Show files or tail page, reload on browser's back button
function showPage() {
    WebTail.unread = 0;
    WebTail.focused = true;
    var m;
    if (WebTail.attached !== undefined) {
        m = JSON.stringify({ type: 'detach', channel: WebTail.attached });
        window.console.debug("send: " + m);
        WebTail.ws.send(m);
    }
    if (location.hash === "") {
        $('table.table tbody').find("tr:not(:last)").remove();
        WebTail.file = '';
        titleReset();
        $('#tail-top').find('[rel="title"]')[0].innerHTML = '';
        $('#src').addClass('hide');
        $('#index').removeClass('hide');
        $('table.table thead tr:first').removeClass('hide'); // show header
        m = JSON.stringify({ type: 'attach' })
        window.console.debug("send: " + m);
        WebTail.ws.send(m);
    } else {
        $('#tail-data').text('');
        if (!$('#index').hasClass('hide')) {
            $('#index').addClass('hide');
        }
        let searchParams = new URLSearchParams(window.location.search)
        $('#mask').val(searchParams.get('mask'))
        $('#src').removeClass('hide');
        tail(location.hash.replace(/^#/, ""));
    }
}

// Setup websocket
function connect() {
    try {
        var host = 'ws';
        if (window.location.protocol === 'https:') host = 'wss';
        host = host + '://' + WebTail.uri;
        WebTail.ws = new WebSocket(host);

        WebTail.ws.onopen = function() {
            window.console.debug('Connection opened');
            showPage();
        }

        WebTail.ws.onclose = function(event) {
            if (event.wasClean) {
                window.console.debug('Connection closed clean');
            } else {
                window.console.debug('Connection aborted');
            }
            window.console.debug('Code: ' + event.code + ' reason: ' + event.reason);
            $("#log").text('Connection closed');
            if (WebTail.timer) {
                clearTimeout(WebTail.timer);
            }
            WebTail.timer = setTimeout(connect, WebTail.timeout);
            WebTail.attached = null;
        }

        WebTail.ws.onerror = function(e) {
            window.console.warn("connect error: %o", e);
            $('#log').text(e.name);
        }

        WebTail.ws.onmessage = function(e) {
            window.console.debug("got " + e.data);
            var lines = e.data.split('\n');
            lines.forEach(processLine);
        };

    } catch (e) {
        window.console.log(e);
    }
}

function processLine(l) {
    var m = JSON.parse(l, JSON.dateParser);
    $("#log").text('');

    if (m.type === 'index') {
        showFiles(m.data);
    } else if (m.type === 'detach') {
        // tail detached
        var mc = (m.channel !== undefined) ? m.channel : '';
        if (mc === WebTail.attached) {
            WebTail.attached = null;
        }
    } else if (m.type === 'attach') {
        // tail attached
        WebTail.attached = (m.channel !== undefined) ? m.channel : '';
    } else if (m.type === 'stats') {
        // TODO: stats requested by calling stats() in console
        window.console.log(JSON.stringify(m.data, null, 4))
    } else if (m.type === 'log') {
        processLog(m.data);
    } else if (m.type === 'error') {
        window.console.warn("server error: %o", m);
        $('#log').text(m.data);
    } else {
        window.console.warn("unknown response: %o", m);
    }
}

function processLog(data) {
    var $area = $('#tail-data');
    var str = (data !== undefined) ? data : '';
    var mask = $('#mask').val();
    var container;
    if (mask === '') {
        container = document.createTextNode(str)
    } else {
        container = document.createElement("span");
        var text = document.createTextNode(str);
        container.appendChild(text);
        if (str.search($('#mask').val()) !== -1) {
            container.style.color = "red";
        }
    }
    $area.append(container);
    $area.append("<br />");
    if (!WebTail.focused) {
        titleUnread(++WebTail.unread);
    }
    if (!WebTail.wait) {
        setTimeout(updateScroll, WebTail.every);
        WebTail.wait = true;
    }

}

// code from https://dev.opera.com/articles/fixing-the-scrolltop-bug/
function bodyOrHtml() {
    if ('scrollingElement' in document) {
        return document.scrollingElement;
    }
    // Fallback for legacy browsers
    if (navigator.userAgent.indexOf('WebKit') !== -1) {
        return document.body;
    }
    return document.documentElement;
}
// scroll window to bottom
function updateScroll() {
    if (!WebTail.scrolled) {
        var obj = bodyOrHtml();
        obj.scrollTop = obj.scrollHeight;
    }
    WebTail.wait = false;
}

$(function() {
    // Open websocket on start
    connect();
    WebTail.title = ' - ' + window.location.hostname;
    titleReset();

    $('#flag').click(function() {
        var obj = bodyOrHtml();
        obj.scrollTop = obj.scrollHeight;
    });
});

$(window).scroll(function() {
    var scrollHeight, totalHeight;
    scrollHeight = document.body.scrollHeight;
    totalHeight = window.scrollY + window.innerHeight;
    if (totalHeight >= scrollHeight) {
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
    var reMsAjax = /^\/Date\((d|-|.*)\)[/|\\]$/;

    JSON.dateParser = function(key, value) {
        if (typeof value !== 'string') return value;
        var a = reISO.exec(value);
        if (a) return new Date(value);
        a = reMsAjax.exec(value);
        if (!a) return value;
        var b = a[1].split(/[-+,.]/);
        return new Date(b[0] ? +b[0] : 0 - +b[1]);
    };
}