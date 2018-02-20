// Checkbox to select all.
function toggle(source, elemName) {
    var checkboxes = document.getElementsByName(elemName);
    for (var i = 0, n = checkboxes.length; i < n; i++) {
        checkboxes[i].checked = source.checked;
    }
};

function listAllSelectedCheckBoxes() {
    var checkboxes = $("[type=checkbox]")
    var selected = []
    var len = 0
    for (var i = 0, n = checkboxes.length; i < n; i++) {
        if (checkboxes[i].name != "selectAll" && checkboxes[i].checked) {
            selected[len++] = checkboxes[i]
        }
    }
    return selected
}

function errorHTML(status, errMsg) {
    return "<span class=\"label label-danger\">" + status + "</span><a href=\"javascript:void(0)\" data-toggle=\"tooltip\" data-placement=\"right\" title=\'" + errMsg + "\'><img src=\"/static/help-icon.jpeg\" width=\"20px\" height=\"20px\"></a>"
}

function bootstrap(column, project, cluster, job, taskId) {
    console.log("bootstrap ... ")
    $.ajax({
        url: "/api/bootstrap/" + project + "/" + cluster + "/" + job + "/" + taskId,
        beforeSend: function () {
            column.html("<span class=\"label label-warning\">Bootstrapping</span>")
        },
        success: function (data) {
            column.html("<span class=\"label label-success\">Running</span>");
        },
        error: function (xhr, status, error) {
            column.html(errorHTML("Bootstrap fail", xhr.responseText));
        }
    });
}

function start(column, project, cluster, job, taskId) {
    console.log("start ... ")
    $.ajax({
        url: "/api/start/" + project + "/" + cluster + "/" + job + "/" + taskId,
        beforeSend: function () {
            column.html("<span class=\"label label-warning\">Starting</span>")
        },
        success: function (data) {
            column.html("<span class=\"label label-success\">Running</span>");
        },
        error: function (xhr, status, error) {
            column.html(errorHTML("Start fail", xhr.responseText));
        }
    });
}

function stop(column, project, cluster, job, taskId) {
    console.log("stop ... ")
    $.ajax({
        url: "/api/stop/" + project + "/" + cluster + "/" + job + "/" + taskId,
        beforeSend: function () {
            column.html("<span class=\"label label-warning\">Stopping</span>")
        },
        success: function (data) {
            column.html("<span class=\"label label-danger\">Stopped</span>");
        },
        error: function (xhr, status, error) {
            column.html(errorHTML("Stop fail", xhr.responseText));
        }
    });
}

function restart(column, project, cluster, job, taskId) {
    console.log("restart ... ")
    $.ajax({
        url: "/api/restart/" + project + "/" + cluster + "/" + job + "/" + taskId,
        beforeSend: function () {
            column.html("<span class=\"label label-warning\">Restarting</span>")
        },
        success: function (data) {
            column.html("<span class=\"label label-success\">Running</span>");
        },
        error: function (xhr, status, error) {
            column.html(errorHTML("Restart fail", xhr.responseText));
        }
    });
}

function rolling_update(column, project, cluster, job, taskId) {
    console.log("rolling_update ... ");
    $.ajax({
        url: "/api/rolling_update/" + project + "/" + cluster + "/" + job + "/" + taskId,
        beforeSend: function () {
            column.html("<span class=\"label label-warning\">RollingUpdating</span>")
        },
        success: function (data) {
            column.html("<span class=\"label label-success\">Running</span>");
        },
        error: function (xhr, status, error) {
            column.html(errorHTML("RollingUpdate fail", xhr.responseText));
        }
    });
}

function cleanup(column, project, cluster, job, taskId) {
    console.log("cleanup ... ");
    $.ajax({
        url: "/api/cleanup/" + project + "/" + cluster + "/" + job + "/" + taskId,
        beforeSend: function () {
            column.html("<span class=\"label label-warning\">Cleanuping</span>")
        },
        success: function (data) {
            column.html("<span class=\"label label-default\">NotBootstrap</span>");
        },
        error: function (xhr, status, error) {
            column.html(errorHTML("Cleanup fail", xhr.responseText));
        }
    });
}

function doAction(action) {
    var selected = listAllSelectedCheckBoxes();
    if (selected.length <= 0) {
        alert("Please select at least one task.")
    }
    for (var i = 0, n = selected.length; i < n; i++) {
        var cb = selected[i];
        var taskId = $(cb).parent().parent().find('td:eq(1)').text();
        var project = $("#hiddenProject").val();
        var cluster = $("#hiddenCluster").val();
        var job = $(cb).val();
        var statusColumn = $(cb).parent().parent().find('td:eq(4)');
        if (action == "bootstrap") {
            bootstrap(statusColumn, project, cluster, job, taskId);
        } else if (action == "start") {
            start(statusColumn, project, cluster, job, taskId);
        } else if (action == "stop") {
            stop(statusColumn, project, cluster, job, taskId)
        } else if (action == "restart") {
            restart(statusColumn, project, cluster, job, taskId)
        } else if (action == "rolling_update") {
            rolling_update(statusColumn, project, cluster, job, taskId)
        } else if (action == "cleanup") {
            cleanup(statusColumn, project, cluster, job, taskId)
        } else {
            alert("Unknown button: " + action);
        }
    }
}

$(document).on("click", "#bootstrapBtn", function () {
    doAction("bootstrap")
});
$(document).on("click", "#startBtn", function () {
    doAction("start")
});
$(document).on("click", "#stopBtn", function () {
    doAction("stop")
});
$(document).on("click", "#restartBtn", function () {
    doAction("restart")
});
$(document).on("click", "#rollingUpdateBtn", function () {
    doAction("rolling_update")
});
$(document).on("click", "#cleanupBtn", function () {
    doAction("cleanup")
});

$(document).ready(function () {
    $('[data-toggle="tooltip"]').tooltip();
});
