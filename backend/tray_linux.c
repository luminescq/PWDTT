#include "tray_linux.h"
#include <libayatana-appindicator/app-indicator.h>
#include <gtk/gtk.h>

static AppIndicator *indicator = NULL;
static GtkWidget *menu = NULL;

extern void onShowClicked();
extern void onQuitClicked();

static void show_cb(GtkMenuItem *item, gpointer data) { onShowClicked(); }
static void quit_cb(GtkMenuItem *item, gpointer data) { onQuitClicked(); }

void wdtt_tray_init(const char *icon_path) {
    indicator = app_indicator_new("pwdtt", icon_path, APP_INDICATOR_CATEGORY_APPLICATION_STATUS);
    app_indicator_set_status(indicator, APP_INDICATOR_STATUS_ACTIVE);

    menu = gtk_menu_new();

    GtkWidget *show_item = gtk_menu_item_new_with_label("\xd0\x9f\xd0\xbe\xd0\xba\xd0\xb0\xd0\xb7\xd0\xb0\xd1\x82\xd1\x8c");
    g_signal_connect(show_item, "activate", G_CALLBACK(show_cb), NULL);
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), show_item);

    GtkWidget *sep = gtk_separator_menu_item_new();
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), sep);

    GtkWidget *quit_item = gtk_menu_item_new_with_label("\xd0\x92\xd1\x8b\xd1\x85\xd0\xbe\xd0\xb4");
    g_signal_connect(quit_item, "activate", G_CALLBACK(quit_cb), NULL);
    gtk_menu_shell_append(GTK_MENU_SHELL(menu), quit_item);

    gtk_widget_show_all(menu);
    app_indicator_set_menu(indicator, GTK_MENU(menu));
}

void wdtt_tray_set_visible(int visible) {
    if (indicator == NULL) return;
    app_indicator_set_status(indicator,
        visible ? APP_INDICATOR_STATUS_ACTIVE : APP_INDICATOR_STATUS_PASSIVE);
}

void wdtt_gtk_main(void) {
    gtk_main();
}
