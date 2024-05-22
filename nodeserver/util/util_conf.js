let confdata = {};

function setConfValue(f, v) {
  confdata[f] = v;
}

function parseLine(str) {
  let reg = /([\w-]*)=(.*)/;
  let sarr = reg.exec(str);
  if (sarr && sarr.length === 3) {
    setConfValue(sarr[1], sarr[2]);
  }
}

function ReadConf(fname) {
  const fs = require('fs');
  const data = fs.readFileSync(fname, 'utf8');
  const lines = data.split('\n');

  lines.forEach((line) => {
    if (line.length > 0 && line[0] !== '#') {
      parseLine(line);
    }
  });
}

function GetConfValue(f) {
	return confdata[f]
}

module.exports = {
	ReadConf,
	GetConfValue,
}