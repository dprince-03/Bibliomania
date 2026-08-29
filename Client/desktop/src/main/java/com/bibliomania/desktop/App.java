package com.bibliomania.desktop;

import javafx.application.Application;
import javafx.scene.Scene;
import javafx.scene.control.Label;
import javafx.scene.layout.StackPane;
import javafx.stage.Stage;

public class App extends Application {

    @Override
    public void start(Stage stage) {
        var root = new StackPane(new Label("Bibliomania Desktop"));
        stage.setScene(new Scene(root, 640, 480));
        stage.setTitle("Bibliomania");
        stage.show();
    }

    public static void main(String[] args) {
        launch(args);
    }
}
