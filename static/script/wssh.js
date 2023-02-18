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

    term.writeln('Connecting ...');
}

function decode(data) {
    return new TextDecoder().decode(data);
}

function encode(data) {
    return new TextEncoder().encode(data);
}

init();

ws.onopen = function () {
    term.onData(function (data) {
        if (ws.readyState === ws.OPEN) {
            ws.send(encode("\x02" + data));
        }
    });

    term.onResize(function (evt) {
        ws.send(encode("\x03" + JSON.stringify({
            cols: evt.cols,
            rows: evt.rows
        })));
    });

    ws.onmessage = function (evt) {
        if (evt.data instanceof ArrayBuffer) {
            const str = decode(evt.data),
                flag = str.substring(0, 1),
                msg = str.substring(1);

            switch (flag) {
                case "\x02":
                    term.write(msg);
                    break;
                case "\x04":
                    console.log(msg)
                    break;
            }
        } else {
            // term.writeln('');
            // term.writeln('The message is not binary data');
            console.warn(evt.data);
        }
    };

    let timer = window.setInterval(function () {
        ws.send(encode("\x04" + "ping"));
    }, 1000 * 60 * 9);

    ws.onclose = function () {
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

let timer = 0;
window.addEventListener('resize', function () {
    clearTimeout(timer);
    timer = setTimeout(function () {
        term.resize(Math.floor(document.body.clientWidth / 9), Math.floor(document.body.clientHeight / 17));
    }, 200);
});
