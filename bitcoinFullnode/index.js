var http = require('http');
const cron = require("node-cron");
const Slack = require('slack-node');
const webhookUri = 'https://hooks.slack.com/services/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';
const slack = new Slack();
slack.setWebhook(webhookUri);
var httpServer = http.createServer(function (req, res) {
	dealWithHTTP(req, res);
}).listen("9485", "127.0.0.1", 511, function (err) {
	if(err) {
		console.log("HTTP ERROR:-", err);
	}
});

var timeStart = new Date();
var timeCheckAlive = new Date();
var alertBlock = 15;
var alertAlive = 30;
var timeSpace = 5;

function dealWithHTTP(req, res){
	const { headers, method, url } = req;
	if(url === '/block'){
		console.log("new block")
		res.statusCode = 200;
		timeStart = new Date();
		alertBlock = 15;
	}
	res.statusCode=404;
	res.end();
}

// schedule tasks to be run on the server
cron.schedule("* * * * *", function() {
	let now = new Date();
      	let seconds = (now - timeStart) / 1000;
	let minutes = seconds / 60;
	if(minutes >= alertBlock) {
		postToSlack("no block in " + alertBlock + " minutes");
		alertBlock += timeSpace;
	}
	// sever still alive
	seconds =  (now - timeCheckAlive) / 1000;
	minutes = seconds / 60;
	if(minutes >= alertAlive) {
		timeCheckAlive = now;
		postToSlack("I'm still alive");
	}
});

function postToSlack(message) {
	slack.webhook({
        	text: message
   	}, function(err, response) {
       		console.log(err, response);
	});

}
