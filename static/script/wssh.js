const term = new window.Terminal(),
    ws = new WebSocket('ws://' + location.host + '/ssh');

term.open(document.getElementById('terminal'));
term.resize(80, 30);
ws.binaryType = 'arraybuffer';

function init() {
    if (term._initialized) {
        return;
    }

    term._initialized = true;

    term.writeln('Welcome to WSSH');
    term.writeln('Connecting ...');
}

function decode(data) {
    return new TextDecoder().decode(data);
}

function encode(data) {
    return new TextEncoder().encode(data);
}

init();

ws.onopen = function (evt) {
    term.onData(function (data) {
        if (ws.readyState === ws.OPEN) {
            ws.send(encode("\x00" + data));
        }
    });

    term.onResize(function (evt) {
        ws.send(encode("\x01" + JSON.stringify({
            cols: evt.cols,
            rows: evt.rows
        })));
    });

    ws.onmessage = function (evt) {
        if (evt.data instanceof ArrayBuffer) {
            const str = decode(evt.data),
                flag = str.substr(0, 1),
                msg = str.substr(1);

            if (flag === "\x00") {
                term.write(msg);
            } else if (flag === "\x02") {
                console.log(msg)
            }
        } else {
            // term.writeln('');
            // term.writeln('The message is not binary data');
            console.warn(evt.data);
        }
    };

    let timer = window.setInterval(function () {
        ws.send(encode("\x02" + "ping"));
    }, 1000 * 60 * 9);

    ws.onclose = function (evt) {
        window.clearInterval(timer);
        term.writeln('');
        term.writeln("Session terminated!");
    }
};

ws.onerror = function (evt) {
    if (typeof console.log == "function") {
        term.writeln('Connection Refused!');
        console.log(evt)
    }
};
