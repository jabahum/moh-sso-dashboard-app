<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true; section>
    <#if section = "form">
        <form id="kc-form-login" class="moh-form" action="${url.loginAction}" method="post">
            <div class="moh-field">
                <label for="username" class="moh-label">
                    <#if !realm.loginWithEmailAllowed>
                        ${msg("username")}
                    <#elseif !realm.registrationEmailAsUsername>
                        ${msg("usernameOrEmail")}
                    <#else>
                        ${msg("email")}
                    </#if>
                </label>
                <input
                    id="username"
                    name="username"
                    type="text"
                    class="moh-input"
                    value="${(login.username!'')}"
                    autocomplete="username"
                    autofocus
                />
            </div>

            <div class="moh-field">
                <div class="moh-label-row">
                    <label for="password" class="moh-label">${msg("password")}</label>
                    <#if realm.resetPasswordAllowed>
                        <a class="moh-inline-link" href="${url.loginResetCredentialsUrl}">
                            ${msg("doForgotPassword")}
                        </a>
                    </#if>
                </div>

                <div class="moh-password-wrap">
                    <input
                        id="password"
                        name="password"
                        type="password"
                        class="moh-input"
                        autocomplete="current-password"
                    />
                    <button
                        type="button"
                        class="moh-password-toggle"
                        data-password-toggle
                        data-password-target="password"
                        aria-label="${msg("showPassword")}"
                    >
                        ${msg("showPassword")}
                    </button>
                </div>
            </div>

            <div class="moh-form-options">
                <#if realm.rememberMe && !usernameEditDisabled??>
                    <label class="moh-checkbox">
                        <input id="rememberMe" name="rememberMe" type="checkbox"
                               <#if login.rememberMe??>checked</#if> />
                        <span>${msg("rememberMe")}</span>
                    </label>
                </#if>
            </div>

            <div class="moh-actions">
                <button class="moh-primary-btn" name="login" id="kc-login" type="submit">
                    ${msg("doLogIn")}
                </button>
            </div>

            <#if social.providers?? && social.providers?size gt 0>
                <div class="moh-divider"><span>or continue with</span></div>

                <div class="moh-idp-grid">
                    <#list social.providers as p>
                        <a
                            id="social-${p.alias}"
                            class="moh-idp-btn"
                            type="button"
                            href="${p.loginUrl}"
                        >
                            ${p.displayName}
                        </a>
                    </#list>
                </div>
            </#if>
        </form>
    </#if>
</@layout.registrationLayout>
