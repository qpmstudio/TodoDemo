import { useAuth } from '../context/AuthContext';

export function Header() {
  const { user, logout } = useAuth();

  return (
    <header className="header">
      <h1 className="header__logo">TodoDemo</h1>
      <div className="header__user">
        {user && (
          <>
            <img
              src={user.github_avatar_url}
              alt={user.display_name}
              className="header__avatar"
            />
            <span className="header__name">{user.display_name}</span>
            <button onClick={logout} className="header__logout-btn">
              Logout
            </button>
          </>
        )}
      </div>
    </header>
  );
}
