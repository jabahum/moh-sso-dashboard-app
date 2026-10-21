<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true; section>
    <#if section = "form">
        <form id="kc-passwd-update-form" class="moh-form" action="${url.loginAction}" method="post">
            <div class="moh-page-intro">
                <h3>${msg("updatePasswordTitle")}</h3>
                <p>${msg("updatePasswordInstruction")}</p>
            </div>

            <div class="moh-field">
                <label for="password-new" class="moh-label">${msg("passwordNew")}</label>
                <div class="moh-password-wrap">
                    <input
                        id="password-new"
                        name="password-new"
                        type="password"
                        class="moh-input"
                        autocomplete="new-password"
                        autofocus
                    />
                    <button
                        type="button"
                        class="moh-password-toggle"
                        data-password-toggle
                        data-password-target="password-new"
                        aria-label="${msg("showPassword")}"
                    >
                        ${msg("showPassword")}
                    </button>
                </div>
            </div>

            <div class="moh-field">
                <label for="password-confirm" class="moh-label">${msg("passwordConfirm")}</label>
                <div class="moh-password-wrap">
                    <input
                        id="password-confirm"
                        name="password-confirm"
                        type="password"
                        class="moh-input"
                        autocomplete="new-password"
                    />
                    <button
                        type="button"
                        class="moh-password-toggle"
                        data-password-toggle
                        data-password-target="password-confirm"
                        aria-label="${msg("showPassword")}"
                    >
                        ${msg("showPassword")}
                    </button>
                </div>
            </div>

            <#if logoutSessions??>
                <label class="moh-checkbox">
                    <input type="checkbox" id="logout-sessions" name="logout-sessions" value="on" checked />
                    <span>${msg("logoutOtherSessions")}</span>
                </label>
            </#if>

            <div class="moh-actions">
                <button class="moh-primary-btn" type="submit">
                    ${msg("doSubmit")}
                </button>
                <#if isAppInitiatedAction??>
                    <button class="moh-secondary-btn" type="submit" name="cancel-aia" value="true">
                        ${msg("doCancel")}
                    </button>
                </#if>
            </div>
        </form>
    </#if>
</@layout.registrationLayout>
